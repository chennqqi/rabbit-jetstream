param(
    [string]$SDKPath = ""
)

$ErrorActionPreference = 'Stop'
$RepositoryRoot = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
if (-not $SDKPath) { $SDKPath = Join-Path $RepositoryRoot 'outlink\rabbit-jetstream-go' }
$SDKPath = (Resolve-Path -LiteralPath $SDKPath).Path
$Project = "rjs-native-sdk-$PID"
$Compose = Join-Path $RepositoryRoot 'deploy\compose\standalone.yml'
$GoImage = 'golang@sha256:81dc45d05a7444ead8c92a389621fafabc8e40f8fd1a19d7e5df14e61e98bc1a'
$GoBuildCache = 'rabbit-jetstream-go-build-cache'
$HostGoModCache = (& go env GOMODCACHE).Trim()
if ($LASTEXITCODE -ne 0 -or -not (Test-Path -LiteralPath $HostGoModCache)) { throw 'host Go module cache is unavailable' }
$PreviousToken = $env:RJS_ADMIN_TOKEN

try {
    $env:RJS_ADMIN_TOKEN = 'native-sdk-test-token'
    docker compose -p $Project -f $Compose up -d --build --wait
    if ($LASTEXITCODE -ne 0) { throw 'standalone deployment failed' }
    $applyOutput = docker run --rm --network "${Project}_default" -e RJS_ADMIN_TOKEN=native-sdk-test-token -e GOPROXY=off -v "${HostGoModCache}:/go/pkg/mod" -v "${GoBuildCache}:/root/.cache/go-build" -v "${RepositoryRoot}:/server" -w /server $GoImage go run ./tools/rjsctl queue apply --url http://management:8223 tests/fixtures/queue-priority.yaml
    if ($LASTEXITCODE -ne 0) { throw "priority Queue apply failed: $($applyOutput -join [Environment]::NewLine)" }
    $apply = $applyOutput | ConvertFrom-Json
    if (-not $apply.revision) { throw 'priority Queue apply returned no revision' }
    docker run --rm --add-host host.docker.internal:host-gateway -e GOPROXY=off -e NATS_URL=nats://host.docker.internal:4222 -e RJS_MANAGED_QUEUE=priority_orders -e RJS_MANAGED_REVISION=$apply.revision -v "${HostGoModCache}:/go/pkg/mod" -v "${GoBuildCache}:/root/.cache/go-build" -v "${SDKPath}:/sdk" -w /sdk $GoImage go test -tags=integration -run=TestIntegrationManagedPriorityQueue -count=1 -v ./...
    if ($LASTEXITCODE -ne 0) { throw 'managed native SDK contract test failed' }
} finally {
    $env:RJS_ADMIN_TOKEN = $PreviousToken
    docker compose -p $Project -f $Compose down -v --remove-orphans 2>$null | Out-Null
}
