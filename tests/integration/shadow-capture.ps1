$ErrorActionPreference = 'Stop'
$RepositoryRoot = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
$Suffix = "$PID-$([Guid]::NewGuid().ToString('N').Substring(0,8))"
$Network = "rjs-shadow-$Suffix"
$Rabbit = "rjs-shadow-rabbit-$Suffix"
$NATS = "rjs-shadow-nats-$Suffix"
$RabbitCapture = "rjs-shadow-rabbit-capture-$Suffix"
$NATSCapture = "rjs-shadow-nats-capture-$Suffix"
$Output = Join-Path $RepositoryRoot ".tmp-rjs-shadow-$Suffix"

function Invoke-Docker { & docker @args; if ($LASTEXITCODE -ne 0) { throw "docker command failed: docker $args" } }
function Wait-Container($Name) { $Code = (& docker wait $Name).Trim(); if ($Code -ne '0') { & docker logs $Name; throw "$Name exited with $Code" } }

Push-Location $RepositoryRoot
try {
    New-Item -ItemType Directory -Path $Output | Out-Null
    Invoke-Docker network create $Network
    Invoke-Docker build -f packaging/Dockerfile.nats-server -t rabbit-jetstream/nats-server:shadow-test .
    Invoke-Docker build -f packaging/Dockerfile.operator -t rabbit-jetstream/operator:shadow-test .
    Invoke-Docker run -d --name $Rabbit --network $Network --network-alias rabbit -e RABBITMQ_DEFAULT_USER=rjs -e RABBITMQ_DEFAULT_PASS=test 'rabbitmq@sha256:5733d284ee87779d6f7628382cc69457d5ac82447c9e8ffba0cfeb92353e343b'
    Invoke-Docker run -d --name $NATS --network $Network --network-alias nats rabbit-jetstream/nats-server:shadow-test -js
    for ($i=0; $i -lt 60; $i++) { & docker exec $Rabbit rabbitmq-diagnostics -q ping 2>$null; if ($LASTEXITCODE -eq 0) { break }; Start-Sleep -Seconds 1 }
    if ($i -eq 60) { throw 'RabbitMQ did not become ready' }
    for ($i=0; $i -lt 60; $i++) { & docker run --rm --network $Network natsio/nats-box@sha256:ffce8bd103383f179f8c7f11cf645726acf5d17280706c530c3b342dbe16334c nats server check connection -s nats://nats:4222 2>$null; if ($LASTEXITCODE -eq 0) { break }; Start-Sleep -Seconds 1 }
    if ($i -eq 60) { throw 'NATS did not become ready' }
    Invoke-Docker run --rm --network $Network -v "${RepositoryRoot}:/src" -w /src golang:1.25-bookworm go run ./tests/helpers/shadow-publisher --mode setup
    $ContainerOutput = "/src/$([IO.Path]::GetFileName($Output))"
    Invoke-Docker run -d --name $RabbitCapture --network $Network -e RJS_RABBITMQ_URL=amqp://rjs:test@rabbit:5672/ -v "${RepositoryRoot}:/src" -w /src rabbit-jetstream/operator:shadow-test migrate capture rabbitmq --queue shadow.capture --output "$ContainerOutput/rabbit.ndjson" --count 3 --timeout 30s
    Invoke-Docker run -d --name $NATSCapture --network $Network -e RJS_NATS_URL=nats://nats:4222 -v "${RepositoryRoot}:/src" -w /src rabbit-jetstream/operator:shadow-test migrate capture jetstream --stream SHADOW --filter shadow.events --output "$ContainerOutput/jetstream.ndjson" --count 3 --timeout 30s
    Invoke-Docker run --rm --network $Network -e RJS_RABBITMQ_URL=amqp://rjs:test@rabbit:5672/ -e RJS_NATS_URL=nats://nats:4222 -v "${RepositoryRoot}:/src" -w /src rabbit-jetstream/operator:shadow-test migrate dual-write --input tests/fixtures/migration-dualwrite.ndjson --journal "$ContainerOutput/dualwrite.ndjson" --exchange shadow.events --routing-key events --subject shadow.events
    Wait-Container $RabbitCapture
    Wait-Container $NATSCapture
    Invoke-Docker run --rm --network $Network -e RJS_RABBITMQ_URL=amqp://rjs:test@rabbit:5672/ -e RJS_NATS_URL=nats://nats:4222 -v "${RepositoryRoot}:/src" -w /src rabbit-jetstream/operator:shadow-test migrate dual-write --input tests/fixtures/migration-dualwrite.ndjson --journal "$ContainerOutput/dualwrite.ndjson" --exchange shadow.events --routing-key events --subject shadow.events
    $RabbitMessages = (& docker exec $Rabbit rabbitmqctl list_queues name messages --formatter json | ConvertFrom-Json | Where-Object { $_.name -eq 'shadow.capture' }).messages
    if ($RabbitMessages -ne 0) { throw "dual-write resume republished $RabbitMessages RabbitMQ messages" }
    $JournalEvents = @(Get-Content -LiteralPath (Join-Path $Output 'dualwrite.ndjson')).Count
    if ($JournalEvents -ne 6) { throw "dual-write journal contains $JournalEvents events, expected 6" }
    Invoke-Docker run --rm -v "${RepositoryRoot}:/src" -w /src rabbit-jetstream/operator:shadow-test migrate reconcile --source "$ContainerOutput/rabbit.ndjson" --target "$ContainerOutput/jetstream.ndjson" --output "$ContainerOutput/report.json" --min-source 3
    $Report = Get-Content -LiteralPath (Join-Path $Output 'report.json') -Raw | ConvertFrom-Json
    if (-not $Report.passed -or $Report.matched -ne 3) { throw 'shadow reconciliation report is incorrect' }
} finally {
    & docker rm -f $RabbitCapture $NATSCapture $Rabbit $NATS 2>$null | Out-Null
    & docker network rm $Network 2>$null | Out-Null
    Remove-Item -LiteralPath $Output -Recurse -Force -ErrorAction SilentlyContinue
    Pop-Location
}
