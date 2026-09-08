param(
    [string]$Version = 'v0.1.0-rc.2',
    [string]$SDKPath = '',
    [string]$Evidence = 'artifacts/local-rc.json',
    [string]$OutputRoot = 'dist'
)

$ErrorActionPreference = 'Stop'
$RepositoryRoot = (Resolve-Path (Join-Path $PSScriptRoot '../..')).Path
if ($Version -notmatch '^v[0-9]+\.[0-9]+\.[0-9]+(?:-[0-9A-Za-z.-]+)?$') { throw "invalid version: $Version" }
if (-not $SDKPath) { $SDKPath = Join-Path $RepositoryRoot 'outlink/rabbit-jetstream-go' }
$SDKPath = (Resolve-Path -LiteralPath $SDKPath).Path
$EvidencePath = if ([IO.Path]::IsPathRooted($Evidence)) { $Evidence } else { Join-Path $RepositoryRoot $Evidence }
$EvidencePath = (Resolve-Path -LiteralPath $EvidencePath).Path
$OutputRootPath = if ([IO.Path]::IsPathRooted($OutputRoot)) { $OutputRoot } else { Join-Path $RepositoryRoot $OutputRoot }
$Destination = Join-Path $OutputRootPath $Version
if (Test-Path -LiteralPath $Destination) { throw "output already exists: $Destination" }

function Get-GitValue([string]$Directory, [string[]]$Arguments) {
    Push-Location $Directory
    try {
        $Value = & git @Arguments
        if ($LASTEXITCODE -ne 0) { throw "git $($Arguments -join ' ') failed in $Directory" }
        return ($Value -join "`n").Trim()
    } finally { Pop-Location }
}

function Invoke-Checked([string]$Command, [string[]]$Arguments) {
    & $Command @Arguments
    if ($LASTEXITCODE -ne 0) { throw "command failed: $Command $($Arguments -join ' ')" }
}

function Get-RelativePath([string]$BasePath, [string]$TargetPath) {
    $BaseFull = [IO.Path]::GetFullPath($BasePath).TrimEnd('\', '/') + [IO.Path]::DirectorySeparatorChar
    $TargetFull = [IO.Path]::GetFullPath($TargetPath)
    $Relative = ([Uri]$BaseFull).MakeRelativeUri([Uri]$TargetFull).ToString()
    return [Uri]::UnescapeDataString($Relative).Replace('/', [IO.Path]::DirectorySeparatorChar)
}

$ServerRevision = Get-GitValue $RepositoryRoot @('rev-parse', 'HEAD')
$SDKRevision = Get-GitValue $SDKPath @('rev-parse', 'HEAD')
if ((Get-GitValue $RepositoryRoot @('status', '--porcelain')) -or (Get-GitValue $SDKPath @('status', '--porcelain'))) { throw 'both repositories must be clean' }
$SDKContract = Get-Content -LiteralPath (Join-Path $SDKPath 'contract.go') -Raw
$SDKVersion = [regex]::Match($SDKContract, 'SDKVersion\s+=\s+"([^"]+)"').Groups[1].Value
$ContractVersion = [regex]::Match($SDKContract, 'ContractVersion\s+=\s+"([^"]+)"').Groups[1].Value
if ($SDKVersion -ne $Version.Substring(1)) { throw "SDK version $SDKVersion does not match $Version" }
$NATSLock = Get-Content -LiteralPath (Join-Path $RepositoryRoot 'upstream/nats-server.lock.json') -Raw | ConvertFrom-Json
$LocalEvidence = Get-Content -LiteralPath $EvidencePath -Raw | ConvertFrom-Json
if ($LocalEvidence.mode -ne 'release' -or $LocalEvidence.server.revision -ne $ServerRevision -or $LocalEvidence.server.dirty -or $LocalEvidence.sdk.revision -ne $SDKRevision -or $LocalEvidence.sdk.dirty) {
    throw 'local Release evidence does not bind the clean current server and SDK revisions'
}
if (@($LocalEvidence.steps | Where-Object status -ne 'passed').Count -ne 0) { throw 'local Release evidence contains failed steps' }

New-Item -ItemType Directory -Force -Path (Join-Path $Destination 'bin/linux-amd64'), (Join-Path $Destination 'bin/linux-arm64'), (Join-Path $Destination 'images'), (Join-Path $Destination 'evidence') | Out-Null
$GoImage = 'golang:1.25.13-alpine@sha256:1e0126852075c9c60731c8ba49088448b91f63e2aed97ca9d1a9791622a05946'
$HostGoModCache = (& go env GOMODCACHE).Trim()
if ($LASTEXITCODE -ne 0) { throw 'cannot locate Go module cache' }
foreach ($Architecture in @('amd64', 'arm64')) {
    $BuildArgs = @('run', '--rm', '-e', 'GOOS=linux', '-e', "GOARCH=$Architecture", '-e', 'CGO_ENABLED=0', '-e', 'GOPROXY=off', '-v', "${RepositoryRoot}:/src:ro", '-v', "${Destination}:/out", '-v', "${HostGoModCache}:/go/pkg/mod", '-w', '/src', $GoImage, 'go', 'build', '-buildvcs=false', '-trimpath')
    Invoke-Checked 'docker' ($BuildArgs + @("-ldflags=-s -w -X main.version=$Version", '-o', "/out/bin/linux-$Architecture/rjs-management", './management/cmd/rjs-management'))
    Invoke-Checked 'docker' ($BuildArgs + @("-ldflags=-s -w -X main.version=$Version", '-o', "/out/bin/linux-$Architecture/rjsctl", './tools/rjsctl'))
    Invoke-Checked 'docker' ($BuildArgs + @('-ldflags=-s -w', '-o', "/out/bin/linux-$Architecture/nativequal", './tools/nativequal'))
}

$Images = @(
    @{ Name = 'nats'; File = 'packaging/Dockerfile.nats-server'; Tag = "rabbit-jetstream/nats:$Version"; Args = @() },
    @{ Name = 'management'; File = 'packaging/Dockerfile.management'; Tag = "rabbit-jetstream/management:$Version"; Args = @('--build-arg', "VERSION=$Version") },
    @{ Name = 'operator'; File = 'packaging/Dockerfile.operator'; Tag = "rabbit-jetstream/operator:$Version"; Args = @('--build-arg', "VERSION=$Version") }
)
foreach ($Image in $Images) {
    $Archive = Join-Path $Destination "images/$($Image.Name).oci.tar"
    $Arguments = @('buildx', 'build', '--platform', 'linux/amd64,linux/arm64', '--provenance=mode=max', '--sbom=true', '--file', $Image.File, '--tag', $Image.Tag) + $Image.Args + @('--output', "type=oci,dest=$Archive", '.')
    Invoke-Checked 'docker' $Arguments
}

$HelmImage = 'alpine/helm:3.18.4@sha256:e7ecbf4a200dea73d64bfb8cb0936829164945f2b4d02a0274093073ee8d264f'
$ChartVersion = $Version.Substring(1)
Invoke-Checked 'docker' @('run', '--rm', '-v', "${RepositoryRoot}:/src:ro", '-v', "${Destination}:/out", '-w', '/src', $HelmImage, 'package', 'deploy/helm/rabbit-jetstream', '--version', $ChartVersion, '--app-version', $Version, '--destination', '/out')
Copy-Item -LiteralPath $EvidencePath -Destination (Join-Path $Destination 'evidence/local-rc.json')
$PerformancePath = Join-Path $RepositoryRoot $LocalEvidence.performance_report.path
Copy-Item -LiteralPath $PerformancePath -Destination (Join-Path $Destination 'evidence/performance-ci.json')
New-Item -ItemType Directory -Path (Join-Path $Destination 'licenses') | Out-Null
Copy-Item -LiteralPath (Join-Path $RepositoryRoot 'upstream/nats-server/LICENSE') -Destination (Join-Path $Destination 'licenses/NATS-LICENSE')
Copy-Item -LiteralPath (Join-Path $RepositoryRoot 'docs/release-license-records.md') -Destination (Join-Path $Destination 'licenses/release-license-records.md')
Copy-Item -LiteralPath (Join-Path $RepositoryRoot 'docs/release-license-records.zh-CN.md') -Destination (Join-Path $Destination 'licenses/release-license-records.zh-CN.md')

$Artifacts = Get-ChildItem -LiteralPath $Destination -Recurse -File | Sort-Object FullName | ForEach-Object {
    [ordered]@{ path = (Get-RelativePath $Destination $_.FullName).Replace('\', '/'); bytes = $_.Length; sha256 = (Get-FileHash -LiteralPath $_.FullName -Algorithm SHA256).Hash.ToLowerInvariant() }
}
$Manifest = [ordered]@{
    schema = 'rabbit-jetstream.io/release-bundle/v1alpha1'
    version = $Version
    generated_at = (Get-Date).ToUniversalTime().ToString('o')
    server_revision = $ServerRevision
    sdk = [ordered]@{ version = $SDKVersion; revision = $SDKRevision }
    contract_version = $ContractVersion
    nats_version = $NATSLock.tag
    platforms = @('linux/amd64', 'linux/arm64')
    qualification = 'local-release-gates-passed; native-linux-soak-and-canary-required'
    artifacts = $Artifacts
}
$ManifestPath = Join-Path $Destination 'release-manifest.json'
$Manifest | ConvertTo-Json -Depth 8 | Set-Content -LiteralPath $ManifestPath -Encoding utf8
$ChecksumFiles = Get-ChildItem -LiteralPath $Destination -Recurse -File | Sort-Object FullName
$ChecksumLines = foreach ($File in $ChecksumFiles) {
    $Relative = (Get-RelativePath $Destination $File.FullName).Replace('\', '/')
    '{0}  {1}' -f (Get-FileHash -LiteralPath $File.FullName -Algorithm SHA256).Hash.ToLowerInvariant(), $Relative
}
$ChecksumPath = Join-Path $Destination 'SHA256SUMS'
[IO.File]::WriteAllText($ChecksumPath, (($ChecksumLines -join "`n") + "`n"), [Text.Encoding]::ASCII)
Write-Host "Local release bundle created: $Destination"
