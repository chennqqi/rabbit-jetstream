param(
    [string]$BaselineNATSImage = 'rabbit-jetstream/nats-server:local',
    [string]$TargetNATSImage = 'rabbit-jetstream/nats-server:local',
    [string]$BaselineManagementImage = 'rabbit-jetstream/management:local',
    [string]$TargetManagementImage = 'rabbit-jetstream/management:local',
    [switch]$BuildLocal
)

$ErrorActionPreference = 'Stop'
$NATSBoxImage = 'natsio/nats-box@sha256:ffce8bd103383f179f8c7f11cf645726acf5d17280706c530c3b342dbe16334c'
$RepositoryRoot = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
$Project = "rjs-rolling-$PID"
$Compose = 'deploy/compose/cluster.yml'
$Network = "${Project}_default"
$PreviousNATSImage = $env:RJS_NATS_IMAGE
$PreviousManagementImage = $env:RJS_MANAGEMENT_IMAGE

function Invoke-Docker {
    & docker @args
    if ($LASTEXITCODE -ne 0) { throw "docker command failed: docker $args" }
}

function Wait-Healthy([string]$Container) {
    foreach ($Attempt in 1..45) {
        $Status = & docker inspect --format '{{.State.Health.Status}}' $Container 2>$null
        if ($LASTEXITCODE -eq 0 -and $Status -eq 'healthy') { return }
        Start-Sleep -Seconds 1
    }
    throw "$Container did not become healthy"
}

function Wait-ReplicasCurrent([string]$Server) {
    foreach ($Attempt in 1..45) {
        try {
            $Info = & docker run --rm --network $Network $NATSBoxImage nats --server $Server stream info RJS_ROLLING --json 2>$null | ConvertFrom-Json
            if ($LASTEXITCODE -eq 0 -and @($Info.cluster.replicas | Where-Object current).Count -eq 2) { return $Info }
        } catch {}
        Start-Sleep -Seconds 1
    }
    throw 'JetStream replicas did not become current'
}

function Publish-Probe([string]$Server, [string]$Payload) {
    Invoke-Docker run --rm --network $Network $NATSBoxImage nats --server $Server publish rjs.rolling $Payload
}

function Roll-Nodes([string]$Image, [int[]]$Order, [string]$Phase) {
    $env:RJS_NATS_IMAGE = $Image
    foreach ($Index in $Order) {
        Invoke-Docker compose -p $Project -f $Compose up -d --no-deps --force-recreate "nats-$Index"
        Wait-Healthy "${Project}-nats-$Index-1"
        $ConfiguredImage = & docker inspect --format '{{.Config.Image}}' "${Project}-nats-$Index-1"
        if ($LASTEXITCODE -ne 0 -or $ConfiguredImage -ne $Image) { throw "nats-$Index did not switch to $Image" }
        $ServerIndex = if ($Index -eq 1) { 2 } else { 1 }
        $null = Wait-ReplicasCurrent "nats://nats-${ServerIndex}:4222"
        Publish-Probe "nats://nats-${ServerIndex}:4222" "$Phase-node-$Index"
        $Ready = Invoke-RestMethod -Uri 'http://127.0.0.1:8223/readyz' -TimeoutSec 5
        if ($Ready.status -ne 'ready') { throw "management was not ready after $Phase node $Index" }
    }
}

Push-Location $RepositoryRoot
try {
    $env:RJS_NATS_IMAGE = $BaselineNATSImage
    $env:RJS_MANAGEMENT_IMAGE = $BaselineManagementImage
    $UpArgs = @('compose', '-p', $Project, '-f', $Compose, 'up', '-d', '--wait')
    if ($BuildLocal) { $UpArgs += '--build' }
    Invoke-Docker @UpArgs
    Invoke-Docker run --rm --network $Network $NATSBoxImage nats --server nats://nats-1:4222 stream add RJS_ROLLING --subjects rjs.rolling --storage file --replicas 3 --defaults
    Invoke-Docker run --rm --network $Network $NATSBoxImage nats --server nats://nats-1:4222 consumer add RJS_ROLLING ROLLING --filter rjs.rolling --ack explicit --pull --defaults
    Publish-Probe 'nats://nats-1:4222' 'baseline'

    Roll-Nodes $TargetNATSImage @(1, 2, 3) 'upgrade'
    $env:RJS_MANAGEMENT_IMAGE = $TargetManagementImage
    Invoke-Docker compose -p $Project -f $Compose up -d --no-deps --force-recreate management
    Wait-Healthy "${Project}-management-1"
    if ((& docker inspect --format '{{.Config.Image}}' "${Project}-management-1") -ne $TargetManagementImage) { throw 'management did not switch to the target image' }

    Roll-Nodes $BaselineNATSImage @(3, 2, 1) 'rollback'
    $env:RJS_MANAGEMENT_IMAGE = $BaselineManagementImage
    Invoke-Docker compose -p $Project -f $Compose up -d --no-deps --force-recreate management
    Wait-Healthy "${Project}-management-1"
    if ((& docker inspect --format '{{.Config.Image}}' "${Project}-management-1") -ne $BaselineManagementImage) { throw 'management did not return to the baseline image' }

    $Final = Wait-ReplicasCurrent 'nats://nats-1:4222'
    if ($Final.state.messages -ne 7) { throw "rolling drill retained $($Final.state.messages) messages, expected 7" }
    $Consumer = & docker run --rm --network $Network $NATSBoxImage nats --server nats://nats-1:4222 consumer info RJS_ROLLING ROLLING --json | ConvertFrom-Json
    if ($LASTEXITCODE -ne 0 -or $Consumer.num_pending -ne 7) { throw 'Consumer metadata or pending count was not preserved' }
    $Nodes = Invoke-RestMethod -Uri 'http://127.0.0.1:8223/api/v1/nodes' -TimeoutSec 8
    if ($Nodes.status -ne 'available' -or $Nodes.available -ne 3) { throw 'cluster did not fully recover after rollback' }
    Write-Output "rolling upgrade and rollback verified: messages=7 replicas=3"
} finally {
    & docker compose -p $Project -f $Compose down -v --remove-orphans 2>$null | Out-Null
    $env:RJS_NATS_IMAGE = $PreviousNATSImage
    $env:RJS_MANAGEMENT_IMAGE = $PreviousManagementImage
    Pop-Location
}
