$ErrorActionPreference = 'Stop'
$GoToolImage = 'golang@sha256:ea341baa9bd5ba6784f6d7161ace70544349a6242d54d34a0fbfd2c4d51c9d58'
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
    Invoke-Docker run -d --name $Rabbit --network $Network --network-alias rabbit --tmpfs '/var/lib/rabbitmq:rw,uid=999,gid=999,mode=0700' -e RABBITMQ_DEFAULT_USER=rjs -e RABBITMQ_DEFAULT_PASS=test -e RABBITMQ_ERLANG_COOKIE=rjs-shadow-test-cookie 'rabbitmq@sha256:5733d284ee87779d6f7628382cc69457d5ac82447c9e8ffba0cfeb92353e343b'
    Invoke-Docker run -d --name $NATS --network $Network --network-alias nats rabbit-jetstream/nats-server:shadow-test -js
    for ($i=0; $i -lt 180; $i++) {
        & docker exec $Rabbit rabbitmq-diagnostics -q ping 2>$null
        if ($LASTEXITCODE -eq 0) { break }
        $RabbitState = (& docker inspect --format '{{.State.Status}}' $Rabbit 2>$null).Trim()
        if ($RabbitState -eq 'exited' -or $RabbitState -eq 'dead') {
            & docker logs $Rabbit
            throw "RabbitMQ exited before becoming ready (state: $RabbitState)"
        }
        Start-Sleep -Seconds 1
    }
    if ($i -eq 180) {
        & docker logs $Rabbit
        throw 'RabbitMQ did not become ready within 180 seconds'
    }
    for ($i=0; $i -lt 60; $i++) { & docker run --rm --network $Network natsio/nats-box@sha256:ffce8bd103383f179f8c7f11cf645726acf5d17280706c530c3b342dbe16334c nats server check connection -s nats://nats:4222 2>$null; if ($LASTEXITCODE -eq 0) { break }; Start-Sleep -Seconds 1 }
    if ($i -eq 60) { throw 'NATS did not become ready' }
    Invoke-Docker run --rm --network $Network -v "${RepositoryRoot}:/src" -w /src $GoToolImage go run ./tests/helpers/shadow-publisher --mode setup
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
    Invoke-Docker run --rm -v "${RepositoryRoot}:/src" -w /src rabbit-jetstream/operator:shadow-test migrate reconcile --source "$ContainerOutput/rabbit.ndjson" --target "$ContainerOutput/jetstream.ndjson" --output "$ContainerOutput/report-1.json" --min-source 3
    Start-Sleep -Milliseconds 10
    Invoke-Docker run --rm -v "${RepositoryRoot}:/src" -w /src rabbit-jetstream/operator:shadow-test migrate reconcile --source "$ContainerOutput/rabbit.ndjson" --target "$ContainerOutput/jetstream.ndjson" --output "$ContainerOutput/report-2.json" --min-source 3
    $Report = Get-Content -LiteralPath (Join-Path $Output 'report-2.json') -Raw | ConvertFrom-Json
    if (-not $Report.passed -or $Report.matched -ne 3) { throw 'shadow reconciliation report is incorrect' }
    Set-Content -LiteralPath (Join-Path $Output 'rabbit.route') -Value 'rabbitmq' -NoNewline
    Set-Content -LiteralPath (Join-Path $Output 'jetstream.route') -Value 'jetstream' -NoNewline
    $Plan = [ordered]@{
        schema = 'rabbit-jetstream.io/cutover-plan/v1alpha1'
        migration_id = 'shadow-e2e'
        reconciliation_reports = @(
            [ordered]@{ path = 'report-1.json'; sha256 = (Get-FileHash -Algorithm SHA256 -LiteralPath (Join-Path $Output 'report-1.json')).Hash.ToLowerInvariant() },
            [ordered]@{ path = 'report-2.json'; sha256 = (Get-FileHash -Algorithm SHA256 -LiteralPath (Join-Path $Output 'report-2.json')).Hash.ToLowerInvariant() }
        )
        steps = @(
            [ordered]@{ name = 'route-publishers'; action = @('/bin/cp', "$ContainerOutput/jetstream.route", "$ContainerOutput/active.route"); rollback = @('/bin/cp', "$ContainerOutput/rabbit.route", "$ContainerOutput/active.route"); timeout = '10s'; idempotent = $true },
            [ordered]@{ name = 'route-consumers'; action = @('/bin/cp', "$ContainerOutput/jetstream.route", "$ContainerOutput/consumer.route"); rollback = @('/bin/cp', "$ContainerOutput/rabbit.route", "$ContainerOutput/consumer.route"); timeout = '10s'; idempotent = $true }
        )
    }
    $Plan | ConvertTo-Json -Depth 8 | Set-Content -LiteralPath (Join-Path $Output 'cutover.json') -Encoding utf8
    Invoke-Docker run --rm -v "${RepositoryRoot}:/src" -w /src rabbit-jetstream/operator:shadow-test migrate cutover apply --plan "$ContainerOutput/cutover.json" --journal "$ContainerOutput/cutover.ndjson" --confirm shadow-e2e
    Invoke-Docker run --rm -v "${RepositoryRoot}:/src" -w /src rabbit-jetstream/operator:shadow-test migrate cutover apply --plan "$ContainerOutput/cutover.json" --journal "$ContainerOutput/cutover.ndjson" --confirm shadow-e2e
    if ((Get-Content -LiteralPath (Join-Path $Output 'active.route') -Raw) -ne 'jetstream' -or (Get-Content -LiteralPath (Join-Path $Output 'consumer.route') -Raw) -ne 'jetstream') { throw 'cutover actions did not select JetStream' }
    Invoke-Docker run --rm -v "${RepositoryRoot}:/src" -w /src rabbit-jetstream/operator:shadow-test migrate cutover rollback --plan "$ContainerOutput/cutover.json" --journal "$ContainerOutput/cutover.ndjson" --confirm shadow-e2e
    if ((Get-Content -LiteralPath (Join-Path $Output 'active.route') -Raw) -ne 'rabbitmq' -or (Get-Content -LiteralPath (Join-Path $Output 'consumer.route') -Raw) -ne 'rabbitmq') { throw 'rollback actions did not restore RabbitMQ' }
} finally {
    & docker rm -f $RabbitCapture $NATSCapture $Rabbit $NATS 2>$null | Out-Null
    & docker network rm $Network 2>$null | Out-Null
    Remove-Item -LiteralPath $Output -Recurse -Force -ErrorAction SilentlyContinue
    Pop-Location
}
