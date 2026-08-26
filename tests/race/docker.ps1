param([string]$SDKPath = '')

$ErrorActionPreference = 'Stop'
$RepositoryRoot = (Resolve-Path (Join-Path $PSScriptRoot '../..')).Path
if (-not $SDKPath) { $SDKPath = Join-Path $RepositoryRoot 'outlink/rabbit-jetstream-go' }
$SDKPath = (Resolve-Path -LiteralPath $SDKPath).Path
$GoImage = 'golang@sha256:81dc45d05a7444ead8c92a389621fafabc8e40f8fd1a19d7e5df14e61e98bc1a'
$GoBuildCache = 'rabbit-jetstream-go-race-build-cache'
$HostGoModCache = (& go env GOMODCACHE).Trim()
if ($LASTEXITCODE -ne 0 -or -not (Test-Path -LiteralPath $HostGoModCache)) { throw 'host Go module cache is unavailable' }

function Invoke-LinuxRace([string]$Source, [string]$Workdir) {
    & docker run --rm -e GOPROXY=off -e CGO_ENABLED=1 -v "${HostGoModCache}:/go/pkg/mod" -v "${GoBuildCache}:/root/.cache/go-build" -v "${Source}:${Workdir}:ro" -w $Workdir $GoImage go test -race -count=1 ./...
    if ($LASTEXITCODE -ne 0) { throw "Linux race test failed for $Source" }
}

Invoke-LinuxRace $RepositoryRoot '/server'
Invoke-LinuxRace $SDKPath '/sdk'
