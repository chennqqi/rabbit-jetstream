param(
    [ValidateSet('Quick', 'Full')]
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

Invoke-Checked 'server-test' $RepositoryRoot 'go' @('test', './...')
Invoke-Checked 'server-vet' $RepositoryRoot 'go' @('vet', './...')
Invoke-Checked 'server-build' $RepositoryRoot 'go' @('build', './...')
Invoke-Checked 'sdk-test' $SDKPath 'go' @('test', './...')
Invoke-Checked 'sdk-vet' $SDKPath 'go' @('vet', './...')
Invoke-Checked 'sdk-build' $SDKPath 'go' @('build', './...')

if ($Mode -eq 'Full') {
    Invoke-Checked 'sdk-docker-integration' $SDKPath 'pwsh' @('-NoProfile', '-File', './scripts/test-integration.ps1')
    Invoke-Checked 'server-sdk-contract' $RepositoryRoot 'pwsh' @('-NoProfile', '-File', './tests/integration/native-sdk.ps1')
    foreach ($Scenario in @('standalone', 'api', 'reconcile', 'apply', 'delete', 'audit', 'auth', 'routing', 'dlq', 'metrics', 'diagnostics', 'controller', 'fault')) {
        Invoke-Checked "server-docker-$Scenario" $RepositoryRoot 'pwsh' @('-NoProfile', '-File', './tests/integration/docker-desktop.ps1', '-Scenario', $Scenario)
    }
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
$OutputPath = if ([System.IO.Path]::IsPathRooted($Output)) { $Output } else { Join-Path $RepositoryRoot $Output }
New-Item -ItemType Directory -Force -Path (Split-Path $OutputPath) | Out-Null
$Evidence | ConvertTo-Json -Depth 8 | Set-Content -LiteralPath $OutputPath -Encoding utf8
Write-Host "Local RC checks passed. Evidence: $OutputPath"
