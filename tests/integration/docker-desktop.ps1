param(
    [ValidateSet('standalone', 'fault')]
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
    if ($Scenario -eq 'standalone') {
        $Project = 'rjs-desktop-standalone'
        $Compose = 'deploy/compose/standalone.yml'
        try {
            Invoke-Docker compose -p $Project -f $Compose up -d --build --wait
            $Ready = Invoke-RestMethod -Uri 'http://127.0.0.1:8223/readyz' -TimeoutSec 5
            if ($Ready.status -ne 'ready') { throw 'management API is not ready' }
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
        Invoke-Docker run --rm --network $Network natsio/nats-box:latest nats --server nats://nats-1:4222 stream add RJS_E2E --subjects rjs.e2e --storage file --replicas 3 --defaults
        Invoke-Docker run --rm --network $Network natsio/nats-box:latest nats --server nats://nats-1:4222 publish rjs.e2e before-failure
        Invoke-Docker compose -p $Project -f $Compose stop nats-1
        Start-Sleep -Seconds 5
        Invoke-Docker run --rm --network $Network natsio/nats-box:latest nats --server nats://nats-2:4222 publish rjs.e2e during-failure
        $Ready = Invoke-RestMethod -Uri 'http://127.0.0.1:8223/readyz' -TimeoutSec 5
        if ($Ready.status -ne 'ready') { throw 'management API failed during node outage' }
        Invoke-Docker compose -p $Project -f $Compose start nats-1
        Wait-Healthy "${Project}-nats-1-1"
        $Info = & docker run --rm --network $Network natsio/nats-box:latest nats --server nats://nats-1:4222 stream info RJS_E2E --json | ConvertFrom-Json
        if ($LASTEXITCODE -ne 0) { throw 'stream inspection failed' }
        if ($Info.state.messages -ne 2) { throw "expected 2 messages, got $($Info.state.messages)" }
        $CurrentReplicas = @($Info.cluster.replicas | Where-Object current).Count
        if ($CurrentReplicas -ne 2) { throw "expected 2 current followers, got $CurrentReplicas" }
    } finally {
        Invoke-Docker compose -p $Project -f $Compose down -v --remove-orphans
    }
} finally {
    Pop-Location
}
