param(
    [ValidateSet('standalone', 'api', 'reconcile', 'fault')]
    [string]$Scenario = 'standalone'
)

$ErrorActionPreference = 'Stop'
$RepositoryRoot = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path

function Invoke-Docker {
    & docker @args
    if ($LASTEXITCODE -ne 0) {
        throw "docker command failed: docker $args"
    }
}

function Wait-Healthy([string]$ContainerName) {
    foreach ($Attempt in 1..45) {
        $State = & docker inspect --format '{{.State.Health.Status}}' $ContainerName 2>$null
        if ($LASTEXITCODE -eq 0 -and $State -eq 'healthy') { return }
        Start-Sleep -Seconds 1
    }
    throw "$ContainerName did not become healthy"
}

Push-Location $RepositoryRoot
try {
    if ($Scenario -in @('standalone', 'api', 'reconcile')) {
        $Project = "rjs-desktop-$Scenario"
        $Compose = 'deploy/compose/standalone.yml'
        try {
            Invoke-Docker compose -p $Project -f $Compose up -d --build --wait
            $Ready = Invoke-RestMethod -Uri 'http://127.0.0.1:8223/readyz' -TimeoutSec 5
            if ($Ready.status -ne 'ready') { throw 'management API is not ready' }
            if ($Scenario -eq 'api') {
                $Network = "${Project}_default"
                Invoke-Docker run --rm --network $Network natsio/nats-box:latest nats --server nats://nats:4222 stream add RJS_API --subjects rjs.api --storage file --replicas 1 --defaults
                Invoke-Docker run --rm --network $Network natsio/nats-box:latest nats --server nats://nats:4222 consumer add RJS_API WORKER --filter rjs.api --ack explicit --pull --defaults
                $Cluster = Invoke-RestMethod -Uri 'http://127.0.0.1:8223/api/v1/cluster' -TimeoutSec 5
                $Streams = Invoke-RestMethod -Uri 'http://127.0.0.1:8223/api/v1/streams' -TimeoutSec 5
                $Stream = Invoke-RestMethod -Uri 'http://127.0.0.1:8223/api/v1/streams/RJS_API' -TimeoutSec 5
                $Consumers = Invoke-RestMethod -Uri 'http://127.0.0.1:8223/api/v1/streams/RJS_API/consumers' -TimeoutSec 5
                $Nodes = Invoke-RestMethod -Uri 'http://127.0.0.1:8223/api/v1/nodes' -TimeoutSec 5
                if ($Cluster.account.streams -ne 1) { throw 'cluster stream count is incorrect' }
                if ($Streams.total -ne 1 -or $Streams.items[0].name -ne 'RJS_API') { throw 'stream list is incorrect' }
                if ($Stream.replicas -ne 1) { throw 'stream detail is incorrect' }
                if ($Consumers.total -ne 1 -or $Consumers.items[0].name -ne 'WORKER') { throw 'consumer list is incorrect' }
                if ($Nodes.status -ne 'available' -or $Nodes.available -ne 1) { throw 'standalone node monitoring is incorrect' }
            }
            if ($Scenario -eq 'reconcile') {
                $Network = "${Project}_default"
                $Result = & docker run --rm --network $Network -v "${RepositoryRoot}:/src" -w /src golang:1.25-bookworm go run ./tools/rjsctl queue reconcile --url http://management:8223 tests/fixtures/queue-basic.yaml | ConvertFrom-Json
                if ($LASTEXITCODE -ne 0) { throw 'Linux rjsctl reconcile failed' }
                if ($Result.status -ne 'ready' -or $Result.blocked) { throw 'reconcile did not produce a ready plan' }
                if (@($Result.operations | Where-Object action -eq 'create').Count -ne 2) { throw 'reconcile did not plan two creates' }
                $Streams = Invoke-RestMethod -Uri 'http://127.0.0.1:8223/api/v1/streams' -TimeoutSec 5
                if ($Streams.total -ne 0) { throw 'reconcile unexpectedly wrote JetStream resources' }
            }
            Invoke-Docker compose -p $Project -f $Compose ps
        } finally {
            Invoke-Docker compose -p $Project -f $Compose down -v --remove-orphans
        }
        return
    }

    $Project = 'rjs-desktop-fault'
    $Compose = 'deploy/compose/cluster.yml'
    $Network = "${Project}_default"
    try {
        Invoke-Docker compose -p $Project -f $Compose up -d --build --wait
        $Nodes = Invoke-RestMethod -Uri 'http://127.0.0.1:8223/api/v1/nodes' -TimeoutSec 5
        if ($Nodes.status -ne 'available' -or $Nodes.available -ne 3) { throw 'initial node monitoring is incorrect' }
        Invoke-Docker run --rm --network $Network natsio/nats-box:latest nats --server nats://nats-1:4222 stream add RJS_E2E --subjects rjs.e2e --storage file --replicas 3 --defaults
        Invoke-Docker run --rm --network $Network natsio/nats-box:latest nats --server nats://nats-1:4222 publish rjs.e2e before-failure
        Invoke-Docker compose -p $Project -f $Compose stop nats-1
        Start-Sleep -Seconds 5
        Invoke-Docker run --rm --network $Network natsio/nats-box:latest nats --server nats://nats-2:4222 publish rjs.e2e during-failure
        $Ready = Invoke-RestMethod -Uri 'http://127.0.0.1:8223/readyz' -TimeoutSec 5
        if ($Ready.status -ne 'ready') { throw 'management API failed during node outage' }
        $Nodes = Invoke-RestMethod -Uri 'http://127.0.0.1:8223/api/v1/nodes' -TimeoutSec 8
        if ($Nodes.status -ne 'degraded' -or $Nodes.available -ne 2 -or $Nodes.unavailable -ne 1) { throw 'node outage was not reported correctly' }
        Invoke-Docker compose -p $Project -f $Compose start nats-1
        Wait-Healthy "${Project}-nats-1-1"
        $Info = & docker run --rm --network $Network natsio/nats-box:latest nats --server nats://nats-1:4222 stream info RJS_E2E --json | ConvertFrom-Json
        if ($LASTEXITCODE -ne 0) { throw 'stream inspection failed' }
        if ($Info.state.messages -ne 2) { throw "expected 2 messages, got $($Info.state.messages)" }
        $CurrentReplicas = @($Info.cluster.replicas | Where-Object current).Count
        if ($CurrentReplicas -ne 2) { throw "expected 2 current followers, got $CurrentReplicas" }
        $ManagedStream = Invoke-RestMethod -Uri 'http://127.0.0.1:8223/api/v1/streams/RJS_E2E' -TimeoutSec 5
        if ($ManagedStream.messages -ne 2) { throw 'management API message count is incorrect' }
        if ([string]::IsNullOrWhiteSpace($ManagedStream.cluster.leader)) { throw 'management API did not report a leader' }
        if (@($ManagedStream.cluster.replicas | Where-Object current).Count -ne 2) { throw 'management API replica state is incorrect' }
        $Nodes = Invoke-RestMethod -Uri 'http://127.0.0.1:8223/api/v1/nodes' -TimeoutSec 8
        if ($Nodes.status -ne 'available' -or $Nodes.available -ne 3) { throw 'recovered node monitoring is incorrect' }
    } finally {
        Invoke-Docker compose -p $Project -f $Compose down -v --remove-orphans
    }
} finally {
    Pop-Location
}
