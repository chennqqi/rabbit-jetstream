param(
    [ValidateSet('ci', 'scale', 'duration', 'soak')][string]$Mode = 'ci',
    [int64]$Messages = 0,
    [TimeSpan]$Duration = [TimeSpan]::Zero,
    [int]$PayloadBytes = 1024,
    [int]$Publishers = 4,
    [int]$Batch = 256,
    [string]$Output = '',
    [string]$Baseline = '',
    [double]$MaxThroughputRegressionPercent = 20,
    [double]$MaxP99RegressionPercent = 30,
    [int]$SampleIntervalSeconds = 5
)
$ErrorActionPreference = 'Stop'
$RepositoryRoot = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
$Suffix = "$PID-$([Guid]::NewGuid().ToString('N').Substring(0,8))"
$Network = "rjs-perf-$Suffix"
$Nodes = @("rjs-perf-1-$Suffix", "rjs-perf-2-$Suffix", "rjs-perf-3-$Suffix")
$Bench = "rjs-perf-bench-$Suffix"
$Temporary = Join-Path $RepositoryRoot ".tmp-rjs-perf-$Suffix"
$ContainerTemporary = "/src/$([IO.Path]::GetFileName($Temporary))"

function Invoke-Docker { & docker @args; if ($LASTEXITCODE -ne 0) { throw "docker command failed: docker $args" } }
function Assert-Report([string]$Path) {
    $Report = Get-Content -LiteralPath $Path -Raw | ConvertFrom-Json
    if ($Report.schema -ne 'rabbit-jetstream.io/performance-report/v1alpha1') { throw 'unknown performance report schema' }
    if ($Report.workload_mode -notin @('count','duration')) { throw 'unknown performance workload mode' }
    if ($Report.replicas -ne 3 -or $Report.requested_messages -lt 1 -or $Report.published -ne $Report.requested_messages -or $Report.consumed -ne $Report.requested_messages) { throw 'incomplete performance workload' }
    if ($Report.missing -ne 0 -or $Report.duplicates -ne 0 -or $Report.corrupt -ne 0) { throw 'message integrity gate failed' }
    if ($Report.publish_messages_per_second -le 0 -or $Report.consume_messages_per_second -le 0 -or $Report.publish_latency_p99_millis -le 0) { throw 'invalid performance measurements' }
    return $Report
}
function Compare-Baseline($Candidate, [string]$Path) {
    $Reference = Assert-Report $Path
    foreach ($Field in @('replicas','payload_bytes','publishers','batch','workload_mode')) { if ($Candidate.$Field -ne $Reference.$Field) { throw "baseline workload differs at $Field" } }
    if ($Candidate.workload_mode -eq 'count' -and $Candidate.requested_messages -ne $Reference.requested_messages) { throw 'baseline message count differs' }
    if ($Candidate.workload_mode -eq 'duration' -and $Candidate.configured_duration_seconds -ne $Reference.configured_duration_seconds) { throw 'baseline duration differs' }
    $MinimumPublish = $Reference.publish_messages_per_second * (1 - $MaxThroughputRegressionPercent / 100)
    $MinimumConsume = $Reference.consume_messages_per_second * (1 - $MaxThroughputRegressionPercent / 100)
    $MaximumP99 = $Reference.publish_latency_p99_millis * (1 + $MaxP99RegressionPercent / 100)
    if ($Candidate.publish_messages_per_second -lt $MinimumPublish) { throw 'publish throughput regression exceeded limit' }
    if ($Candidate.consume_messages_per_second -lt $MinimumConsume) { throw 'consume throughput regression exceeded limit' }
    if ($Candidate.publish_latency_p99_millis -gt $MaximumP99) { throw 'publish P99 regression exceeded limit' }
}

if ($PayloadBytes -lt 16 -or $Publishers -lt 1 -or $Batch -lt 1 -or $SampleIntervalSeconds -lt 1) { throw 'invalid workload parameters' }
switch ($Mode) {
    'ci' { if ($Messages -eq 0) { $Messages = 20000 }; if ($Duration -ne [TimeSpan]::Zero) { throw 'ci mode uses a fixed message count' } }
    'scale' { if ($Messages -eq 0) { $Messages = 1000000 }; if ($Duration -ne [TimeSpan]::Zero) { throw 'scale mode uses a fixed message count' } }
    'duration' {
        if ($Duration -eq [TimeSpan]::Zero) { $Duration = [TimeSpan]::FromMinutes(10) }
        if ($Duration -lt [TimeSpan]::FromSeconds(10) -or $Messages -ne 0) { throw 'duration mode requires at least 10 seconds and does not accept -Messages' }
    }
    'soak' {
        if ($Duration -eq [TimeSpan]::Zero) { $Duration = [TimeSpan]::FromHours(24) }
        if ($Duration -lt [TimeSpan]::FromHours(24)) { throw 'release soak duration must be at least 24 hours' }
        if ($Messages -ne 0 -or $Output -eq '' -or $Baseline -eq '') { throw 'soak mode requires -Output and -Baseline and does not accept -Messages' }
    }
}

Push-Location $RepositoryRoot
try {
    New-Item -ItemType Directory -Path $Temporary | Out-Null
    Invoke-Docker network create $Network
    Invoke-Docker build -f packaging/Dockerfile.nats-server -t rabbit-jetstream/nats-server:performance-test .
    for ($Index = 0; $Index -lt 3; $Index++) {
        $Number = $Index + 1
        $Config = "/src/deploy/nats/cluster-$Number.conf"
        Invoke-Docker run -d --name $Nodes[$Index] --network $Network --network-alias "nats-$Number" -v "${RepositoryRoot}:/src:ro" rabbit-jetstream/nats-server:performance-test -c $Config
    }
    for ($Attempt = 0; $Attempt -lt 90; $Attempt++) {
        & docker run --rm --network $Network natsio/nats-box@sha256:ffce8bd103383f179f8c7f11cf645726acf5d17280706c530c3b342dbe16334c nats server check jetstream -s nats://nats-1:4222 2>$null
        if ($LASTEXITCODE -eq 0) { break }
        Start-Sleep -Seconds 1
    }
    if ($Attempt -eq 90) { throw 'three-node JetStream cluster did not become ready' }
    $Arguments = @('run','-d','--name',$Bench,'--network',$Network,'-v',"${RepositoryRoot}:/src",'-w','/src','golang@sha256:ea341baa9bd5ba6784f6d7161ace70544349a6242d54d34a0fbfd2c4d51c9d58','go','run','./tests/helpers/jetstream-bench','--server','nats://nats-1:4222,nats://nats-2:4222,nats://nats-3:4222','--output',"$ContainerTemporary/report.json",'--payload-bytes',"$PayloadBytes",'--publishers',"$Publishers",'--batch',"$Batch",'--replicas','3')
    if ($Mode -in @('duration','soak')) { $Arguments += @('--messages','0','--duration',"$([int64]$Duration.TotalSeconds)s",'--timeout','30m') } else { $Arguments += @('--messages',"$Messages",'--timeout','10m') }
    Invoke-Docker @Arguments
    $ResourcePath = Join-Path $Temporary 'docker-stats.ndjson'
    while ($true) {
        foreach ($Node in $Nodes) {
            $State = & docker inspect $Node | ConvertFrom-Json
            if (-not $State[0].State.Running -or $State[0].RestartCount -ne 0) { throw "$Node was not continuously healthy" }
            $Stats = & docker stats --no-stream --format '{{json .}}' $Node | ConvertFrom-Json
            [ordered]@{ captured_at = [DateTime]::UtcNow.ToString('o'); node = $Node; cpu = $Stats.CPUPerc; memory = $Stats.MemUsage; memory_percent = $Stats.MemPerc; network_io = $Stats.NetIO; block_io = $Stats.BlockIO; pids = $Stats.PIDs } | ConvertTo-Json -Compress | Add-Content -LiteralPath $ResourcePath -Encoding utf8
        }
        $BenchState = & docker inspect $Bench | ConvertFrom-Json
        if (-not $BenchState[0].State.Running) {
            & docker logs $Bench
            if ($BenchState[0].State.ExitCode -ne 0) { throw "benchmark exited with $($BenchState[0].State.ExitCode)" }
            break
        }
        Start-Sleep -Seconds $SampleIntervalSeconds
    }
    $ReportPath = Join-Path $Temporary 'report.json'
    $Report = Assert-Report $ReportPath
    if ($Baseline -ne '') { Compare-Baseline $Report $Baseline }
    if ($Output -ne '') {
        if (Test-Path -LiteralPath $Output) { throw "output already exists: $Output" }
        Copy-Item -LiteralPath $ReportPath -Destination $Output
        Copy-Item -LiteralPath $ResourcePath -Destination "$Output.resources.ndjson"
    }
    Write-Output ("performance verified: messages={0} publish={1:N0}/s consume={2:N0}/s p99={3:N2}ms" -f $Report.published,$Report.publish_messages_per_second,$Report.consume_messages_per_second,$Report.publish_latency_p99_millis)
} finally {
    & docker rm -f -v $Bench @Nodes 2>$null | Out-Null
    & docker network rm $Network 2>$null | Out-Null
    Remove-Item -LiteralPath $Temporary -Recurse -Force -ErrorAction SilentlyContinue
    Pop-Location
}
