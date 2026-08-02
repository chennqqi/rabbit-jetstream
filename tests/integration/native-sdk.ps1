param(
    [string]$SDKPath = ""
)

$ErrorActionPreference = 'Stop'
$RepositoryRoot = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
if (-not $SDKPath) { $SDKPath = Join-Path $RepositoryRoot 'outlink\rabbit-jetstream-go' }
$SDKPath = (Resolve-Path -LiteralPath $SDKPath).Path
$Project = "rjs-native-sdk-$PID"
$Compose = Join-Path $RepositoryRoot 'deploy\compose\standalone.yml'
$GoImage = 'golang:1.25.0-bookworm'
$PreviousToken = $env:RJS_ADMIN_TOKEN

try {
    $env:RJS_ADMIN_TOKEN = 'native-sdk-test-token'
    docker compose -p $Project -f $Compose up -d --build --wait
    if ($LASTEXITCODE -ne 0) { throw 'standalone deployment failed' }
    $apply = docker run --rm --network "${Project}_default" -e RJS_ADMIN_TOKEN=native-sdk-test-token -v "${RepositoryRoot}:/server" -w /server $GoImage go run ./tools/rjsctl queue apply --url http://management:8223 tests/fixtures/queue-priority.yaml | ConvertFrom-Json
    if ($LASTEXITCODE -ne 0 -or -not $apply.revision) { throw 'priority Queue apply failed' }
    docker run --rm --add-host host.docker.internal:host-gateway -e NATS_URL=nats://host.docker.internal:4222 -e RJS_MANAGED_QUEUE=priority_orders -e RJS_MANAGED_REVISION=$apply.revision -v "${SDKPath}:/sdk" -w /sdk $GoImage go test -tags=integration -run=TestIntegrationManagedPriorityQueue -count=1 -v ./...
    if ($LASTEXITCODE -ne 0) { throw 'managed native SDK contract test failed' }
} finally {
    $env:RJS_ADMIN_TOKEN = $PreviousToken
    docker compose -p $Project -f $Compose down -v --remove-orphans 2>$null | Out-Null
}
