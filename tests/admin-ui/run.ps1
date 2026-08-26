param([string]$Project = 'rjs-admin-ui-e2e')

$ErrorActionPreference = 'Stop'
$RepositoryRoot = (Resolve-Path (Join-Path $PSScriptRoot '../..')).Path
$Compose = Join-Path $RepositoryRoot 'deploy/compose/standalone.yml'
$PreviousToken = $env:RJS_ADMIN_TOKEN
$env:RJS_ADMIN_TOKEN = 'browser-test-token'

try {
    & docker compose -p $Project -f $Compose up -d --build --wait
    if ($LASTEXITCODE -ne 0) { throw 'Admin UI Compose environment failed to start' }
    & docker build -t rabbit-jetstream/admin-ui-e2e:local (Join-Path $RepositoryRoot 'tests/admin-ui')
    if ($LASTEXITCODE -ne 0) { throw 'Admin UI Playwright image failed to build' }
    & docker run --rm --add-host host.docker.internal:host-gateway rabbit-jetstream/admin-ui-e2e:local
    if ($LASTEXITCODE -ne 0) { throw 'Admin UI Playwright tests failed' }
} finally {
    & docker compose -p $Project -f $Compose down -v --remove-orphans
    $env:RJS_ADMIN_TOKEN = $PreviousToken
}
