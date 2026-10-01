param(
    [ValidateSet('Quick', 'Full', 'Release')]
    [string]$Mode = 'Quick',
    [string]$SDKPath = '',
    [string]$Output = 'artifacts/local-rc.json',
    [switch]$AllowDirty
)

$ErrorActionPreference = 'Stop'
$RepositoryRoot = (Resolve-Path (Join-Path $PSScriptRoot '../..')).Path
if (-not $SDKPath) { $SDKPath = Join-Path $RepositoryRoot 'outlink/rabbit-jetstream-go' }
$SDKPath = (Resolve-Path $SDKPath).Path
$Steps = [System.Collections.Generic.List[object]]::new()
$ArtifactsRoot = Join-Path $RepositoryRoot 'artifacts'
New-Item -ItemType Directory -Force -Path $ArtifactsRoot | Out-Null
$ReleaseCacheRoot = Join-Path $ArtifactsRoot 'release-cache'
$env:GOCACHE = Join-Path $ReleaseCacheRoot 'go-build'
$env:XDG_CONFIG_HOME = Join-Path $ReleaseCacheRoot 'xdg'
New-Item -ItemType Directory -Force -Path $env:GOCACHE, $env:XDG_CONFIG_HOME | Out-Null

function Invoke-Checked {
    param([string]$Name, [string]$WorkingDirectory, [string]$Command, [string[]]$Arguments)
    $started = Get-Date
    Push-Location $WorkingDirectory
    try {
        & $Command @Arguments
        if ($LASTEXITCODE -ne 0) { throw "$Name failed with exit code $LASTEXITCODE" }
    } finally {
        Pop-Location
    }
    $Steps.Add([ordered]@{ name = $Name; status = 'passed'; duration_seconds = [math]::Round(((Get-Date) - $started).TotalSeconds, 3) })
}

function Get-GitValue {
    param([string]$WorkingDirectory, [string[]]$Arguments)
    Push-Location $WorkingDirectory
    try {
        $value = (& git @Arguments)
        if ($LASTEXITCODE -ne 0) { throw "git $($Arguments -join ' ') failed" }
        return ($value -join "`n").Trim()
    } finally {
        Pop-Location
    }
}

function Get-RelativePath {
    param([string]$BasePath, [string]$TargetPath)
    $BaseFull = [IO.Path]::GetFullPath($BasePath).TrimEnd('\', '/') + [IO.Path]::DirectorySeparatorChar
    $TargetFull = [IO.Path]::GetFullPath($TargetPath)
    $Relative = ([Uri]$BaseFull).MakeRelativeUri([Uri]$TargetFull).ToString()
    return [Uri]::UnescapeDataString($Relative).Replace('/', [IO.Path]::DirectorySeparatorChar)
}

$ServerRevision = Get-GitValue $RepositoryRoot @('rev-parse', 'HEAD')
$SDKRevision = Get-GitValue $SDKPath @('rev-parse', 'HEAD')
$ServerDirty = [bool](Get-GitValue $RepositoryRoot @('status', '--porcelain'))
$SDKDirty = [bool](Get-GitValue $SDKPath @('status', '--porcelain'))
if (-not $AllowDirty -and ($ServerDirty -or $SDKDirty)) {
    throw 'Both repositories must be clean. Use -AllowDirty only for development runs.'
}

$SDKContract = Get-Content -LiteralPath (Join-Path $SDKPath 'contract.go') -Raw
$SDKVersion = [regex]::Match($SDKContract, 'SDKVersion\s+=\s+"([^"]+)"').Groups[1].Value
$ContractVersion = [regex]::Match($SDKContract, 'ContractVersion\s+=\s+"([^"]+)"').Groups[1].Value
$NATSLock = Get-Content -LiteralPath (Join-Path $RepositoryRoot 'upstream/nats-server.lock.json') -Raw | ConvertFrom-Json
if (-not $SDKVersion -or -not $ContractVersion) { throw 'Unable to read SDK or contract version.' }
if ($NATSLock.tag -ne 'v2.14.1') { throw "Unexpected NATS pin $($NATSLock.tag)" }

$NpmCommand = if ([Environment]::OSVersion.Platform -eq [PlatformID]::Win32NT) { 'npm.cmd' } else { 'npm' }
Invoke-Checked 'admin-ui-clean-install' (Join-Path $RepositoryRoot 'admin-ui') $NpmCommand @('ci', '--ignore-scripts')
Invoke-Checked 'admin-ui-module-tests' (Join-Path $RepositoryRoot 'admin-ui') $NpmCommand @('test')
Invoke-Checked 'admin-ui-candidate-build' (Join-Path $RepositoryRoot 'admin-ui') $NpmCommand @('run', 'build')
Invoke-Checked 'admin-ui-embedded-assets' (Join-Path $RepositoryRoot 'admin-ui') $NpmCommand @('run', 'verify:dist')
Invoke-Checked 'server-test' $RepositoryRoot 'go' @('test', './...')
Invoke-Checked 'server-vet' $RepositoryRoot 'go' @('vet', './...')
Invoke-Checked 'server-build' $RepositoryRoot 'go' @('build', './...')
Invoke-Checked 'sdk-test' $SDKPath 'go' @('test', './...')
Invoke-Checked 'sdk-vet' $SDKPath 'go' @('vet', './...')
Invoke-Checked 'sdk-build' $SDKPath 'go' @('build', './...')

if ($Mode -in @('Full', 'Release')) {
    Invoke-Checked 'admin-ui-browser-standalone-e2e' $RepositoryRoot 'pwsh' @('-NoProfile', '-File', './tests/admin-ui/run.ps1', '-Project', 'rjs-admin-ui-standalone-e2e', '-DeploymentProfile', 'standalone')
    Invoke-Checked 'admin-ui-browser-cluster-e2e' $RepositoryRoot 'pwsh' @('-NoProfile', '-File', './tests/admin-ui/run.ps1', '-Project', 'rjs-admin-ui-cluster-e2e', '-DeploymentProfile', 'cluster')
    Invoke-Checked 'sdk-docker-integration' $SDKPath 'pwsh' @('-NoProfile', '-File', './scripts/test-integration.ps1')
    Invoke-Checked 'server-sdk-contract' $RepositoryRoot 'pwsh' @('-NoProfile', '-File', './tests/integration/native-sdk.ps1', '-SDKPath', $SDKPath)
    foreach ($Scenario in @('standalone', 'api', 'reconcile', 'apply', 'delete', 'audit', 'auth', 'routing', 'dlq', 'metrics', 'diagnostics', 'controller', 'fault')) {
        Invoke-Checked "server-docker-$Scenario" $RepositoryRoot 'pwsh' @('-NoProfile', '-File', './tests/integration/docker-desktop.ps1', '-Scenario', $Scenario)
    }
}

$PerformanceReport = $null
if ($Mode -eq 'Release') {
    Invoke-Checked 'linux-race' $RepositoryRoot 'pwsh' @('-NoProfile', '-File', './tests/race/docker.ps1', '-SDKPath', $SDKPath)
    Invoke-Checked 'coverage-gate' $RepositoryRoot 'pwsh' @('-NoProfile', '-File', './tests/coverage/check.ps1')
    Invoke-Checked 'backup-restore' $RepositoryRoot 'pwsh' @('-NoProfile', '-File', './tests/integration/backup-restore.ps1')
    Invoke-Checked 'rolling-upgrade-rollback' $RepositoryRoot 'pwsh' @('-NoProfile', '-File', './tests/integration/rolling-upgrade.ps1', '-BuildLocal')
    Invoke-Checked 'rabbitmq-migration' $RepositoryRoot 'pwsh' @('-NoProfile', '-File', './tests/integration/migration.ps1')
    $ShadowGoCache = (& go env GOMODCACHE).Trim()
    if ($LASTEXITCODE -ne 0) { throw 'cannot locate shadow helper module cache' }
    Invoke-Checked 'shadow-migration' $RepositoryRoot 'pwsh' @('-NoProfile', '-File', './tests/integration/shadow-capture.ps1', '-GoModCache', $ShadowGoCache)
    Invoke-Checked 'helm-gate' $RepositoryRoot 'pwsh' @('-NoProfile', '-File', './tests/deployment/helm.ps1')
    Invoke-Checked 'security-gate' $RepositoryRoot 'pwsh' @('-NoProfile', '-File', './tests/security/scan.ps1')
    $PerformanceReport = Join-Path $ArtifactsRoot ("performance-ci-{0}.json" -f (Get-Date).ToUniversalTime().ToString('yyyyMMddTHHmmssZ'))
    Invoke-Checked 'performance-ci' $RepositoryRoot 'pwsh' @('-NoProfile', '-File', './tests/performance/jetstream.ps1', '-Mode', 'ci', '-Output', $PerformanceReport)
}

$Evidence = [ordered]@{
    schema = 'rabbit-jetstream.io/local-rc/v1alpha1'
    generated_at = (Get-Date).ToUniversalTime().ToString('o')
    mode = $Mode.ToLowerInvariant()
    server = [ordered]@{ revision = $ServerRevision; dirty = $ServerDirty }
    sdk = [ordered]@{ version = $SDKVersion; revision = $SDKRevision; dirty = $SDKDirty }
    contract_version = $ContractVersion
    nats_version = $NATSLock.tag
    host = [ordered]@{ os = [System.Environment]::OSVersion.ToString(); architecture = [System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture.ToString() }
    steps = $Steps
}
$Evidence['performance_report'] = if ($PerformanceReport) {
    [ordered]@{ path = Get-RelativePath $RepositoryRoot $PerformanceReport; sha256 = (Get-FileHash -LiteralPath $PerformanceReport -Algorithm SHA256).Hash.ToLowerInvariant() }
} else { $null }
$OutputPath = if ([System.IO.Path]::IsPathRooted($Output)) { $Output } else { Join-Path $RepositoryRoot $Output }
New-Item -ItemType Directory -Force -Path (Split-Path $OutputPath) | Out-Null
$Evidence | ConvertTo-Json -Depth 8 | Set-Content -LiteralPath $OutputPath -Encoding utf8
Write-Host "Local RC checks passed. Evidence: $OutputPath"
