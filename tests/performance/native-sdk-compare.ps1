param(
    [int64]$Messages = 20000,
    [int]$PayloadBytes = 1024,
    [int]$Publishers = 4,
    [int]$MaxPriority = 2,
    [string]$SDKPath = '',
    [string]$Output = '',
    [double]$MinPublishThroughputRatio = 0.40,
    [double]$MinConsumeThroughputRatio = 0.05,
    [double]$MaxPublishP99Ratio = 4.0
)

$ErrorActionPreference = 'Stop'
$RepositoryRoot = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
if (-not $SDKPath) { $SDKPath = Join-Path $RepositoryRoot 'outlink\rabbit-jetstream-go' }
$SDKPath = (Resolve-Path -LiteralPath $SDKPath).Path
$Suffix = "$PID-$([Guid]::NewGuid().ToString('N').Substring(0,8))"
$Network = "rjs-sdk-compare-$Suffix"
$Nodes = @(1..3 | ForEach-Object { "rjs-sdk-compare-$($_)-$Suffix" })
$Temporary = Join-Path $RepositoryRoot ".tmp-rjs-sdk-compare-$Suffix"
$GoImage = 'golang:1.25.0-bookworm'
$NATSBox = 'natsio/nats-box@sha256:ffce8bd103383f179f8c7f11cf645726acf5d17280706c530c3b342dbe16334c'

function Invoke-Docker { & docker @args; if ($LASTEXITCODE -ne 0) { throw "docker command failed: docker $args" } }
function Assert-Integrity($Report, [string]$Mode) {
    if ($Report.published -ne $Messages -or $Report.consumed -ne $Messages -or $Report.missing -ne 0 -or $Report.duplicates -ne 0 -or $Report.corrupt -ne 0) { throw "$Mode integrity gate failed" }
    if ($Report.publish_messages_per_second -le 0 -or $Report.consume_messages_per_second -le 0 -or $Report.publish_latency_p99_millis -le 0) { throw "$Mode returned invalid measurements" }
}

if ($Messages -lt 1 -or $PayloadBytes -lt 16 -or $Publishers -lt 1 -or $MaxPriority -lt 0 -or $MaxPriority -gt 255) { throw 'invalid comparison workload' }

try {
    New-Item -ItemType Directory -Path $Temporary | Out-Null
    Invoke-Docker network create $Network
    Invoke-Docker build -f (Join-Path $RepositoryRoot 'packaging\Dockerfile.nats-server') -t rabbit-jetstream/nats-server:sdk-compare $RepositoryRoot
    for ($Index = 0; $Index -lt 3; $Index++) {
        $Number = $Index + 1
        Invoke-Docker run -d --name $Nodes[$Index] --network $Network --network-alias "nats-$Number" -v "${RepositoryRoot}:/src:ro" rabbit-jetstream/nats-server:sdk-compare -c "/src/deploy/nats/cluster-$Number.conf"
    }
    for ($Attempt = 0; $Attempt -lt 90; $Attempt++) {
        & docker run --rm --network $Network $NATSBox nats server check jetstream -s nats://nats-1:4222 2>$null
        if ($LASTEXITCODE -eq 0) { break }
        Start-Sleep -Seconds 1
    }
    if ($Attempt -eq 90) { throw 'three-node JetStream cluster did not become ready' }

    $Servers = 'nats://nats-1:4222,nats://nats-2:4222,nats://nats-3:4222'
    Invoke-Docker run --rm --network $Network -v "${RepositoryRoot}:/src" -v "${Temporary}:/results" -w /src $GoImage go run ./tests/helpers/jetstream-bench --server $Servers --output /results/direct.json --messages $Messages --payload-bytes $PayloadBytes --publishers $Publishers --batch 1 --replicas 3
    Invoke-Docker run --rm --network $Network -v "${SDKPath}:/src" -v "${Temporary}:/results" -w /src $GoImage go run ./cmd/rjs-sdk-bench --server $Servers --output /results/sdk.json --messages $Messages --payload-bytes $PayloadBytes --publishers $Publishers --max-priority $MaxPriority --replicas 3

    $Direct = Get-Content -LiteralPath (Join-Path $Temporary 'direct.json') -Raw | ConvertFrom-Json
    $SDK = Get-Content -LiteralPath (Join-Path $Temporary 'sdk.json') -Raw | ConvertFrom-Json
    Assert-Integrity $Direct 'direct JetStream'
    Assert-Integrity $SDK 'native SDK'
    $PublishRatio = $SDK.publish_messages_per_second / $Direct.publish_messages_per_second
    $ConsumeRatio = $SDK.consume_messages_per_second / $Direct.consume_messages_per_second
    $P99Ratio = $SDK.publish_latency_p99_millis / $Direct.publish_latency_p99_millis
    if ($PublishRatio -lt $MinPublishThroughputRatio) { throw "SDK publish throughput ratio $PublishRatio is below $MinPublishThroughputRatio" }
    if ($ConsumeRatio -lt $MinConsumeThroughputRatio) { throw "SDK consume throughput ratio $ConsumeRatio is below $MinConsumeThroughputRatio" }
    if ($P99Ratio -gt $MaxPublishP99Ratio) { throw "SDK publish P99 ratio $P99Ratio exceeds $MaxPublishP99Ratio" }

    $Comparison = [ordered]@{
        schema = 'rabbit-jetstream.io/client-performance-comparison/v1alpha1'
        captured_at = [DateTime]::UtcNow.ToString('o')
        workload = [ordered]@{ messages=$Messages; payload_bytes=$PayloadBytes; publishers=$Publishers; replicas=3; direct_batch=1; priority_levels=$MaxPriority+1 }
        gates = [ordered]@{ min_publish_throughput_ratio=$MinPublishThroughputRatio; min_consume_throughput_ratio=$MinConsumeThroughputRatio; max_publish_p99_ratio=$MaxPublishP99Ratio }
        ratios = [ordered]@{ publish_throughput=$PublishRatio; consume_throughput=$ConsumeRatio; publish_p99=$P99Ratio }
        direct = $Direct
        sdk = $SDK
    }
    if ($Output) {
        if (Test-Path -LiteralPath $Output) { throw "output already exists: $Output" }
        $Comparison | ConvertTo-Json -Depth 8 | Set-Content -LiteralPath $Output -Encoding utf8NoBOM
    }
    Write-Output ("SDK/direct ratios: publish={0:P1} consume={1:P1} p99={2:N2}x" -f $PublishRatio,$ConsumeRatio,$P99Ratio)
} finally {
    & docker rm -f -v @Nodes 2>$null | Out-Null
    & docker network rm $Network 2>$null | Out-Null
    Remove-Item -LiteralPath $Temporary -Recurse -Force -ErrorAction SilentlyContinue
}
