$ErrorActionPreference = 'Stop'
$RepositoryRoot = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
$TemporaryRoot = Join-Path $RepositoryRoot ".tmp-rjs-migration-$PID"
$Output = Join-Path $TemporaryRoot 'result'
$RootPrefix = [IO.Path]::GetFullPath($RepositoryRoot).TrimEnd([IO.Path]::DirectorySeparatorChar) + [IO.Path]::DirectorySeparatorChar
if (-not [IO.Path]::GetFullPath($TemporaryRoot).StartsWith($RootPrefix, [StringComparison]::OrdinalIgnoreCase)) { throw 'temporary migration path escaped the repository' }

function Invoke-Docker {
    & docker @args
    if ($LASTEXITCODE -ne 0) { throw "docker command failed: docker $args" }
}

Push-Location $RepositoryRoot
try {
    New-Item -ItemType Directory -Path $TemporaryRoot | Out-Null
    if ($IsLinux) { & chmod 0777 $TemporaryRoot }
    Invoke-Docker build -f packaging/Dockerfile.operator -t rabbit-jetstream/operator:migration-test .
    $ContainerOutput = '/output/result'
    $RunUser = @()
    if ($IsLinux) { $RunUser = @('--user', "$(& id -u):$(& id -g)") }
    $Mounts = $RunUser + @('-v', "${RepositoryRoot}:/src:ro", '-v', "${TemporaryRoot}:/output", '-w', '/src')
    Invoke-Docker run --rm @Mounts rabbit-jetstream/operator:migration-test migrate rabbitmq-definitions --output $ContainerOutput tests/fixtures/rabbitmq-definitions.json
    $Report = Get-Content -LiteralPath (Join-Path $Output 'migration-report.json') -Raw | ConvertFrom-Json
    if (-not $Report.compatible -or $Report.converted -ne 3 -or $Report.queues -ne 3) { throw 'migration report is incorrect' }
    foreach ($Name in @('orders', 'orders_dlq', 'audit')) {
        Invoke-Docker run --rm @Mounts rabbit-jetstream/operator:migration-test queue validate "$ContainerOutput/$Name.yaml"
    }
    $PassReport = "$ContainerOutput/reconciliation-pass.json"
    Invoke-Docker run --rm @Mounts rabbit-jetstream/operator:migration-test migrate reconcile --source tests/fixtures/migration-shadow-source.ndjson --target tests/fixtures/migration-shadow-target.ndjson --output $PassReport
    $Pass = Get-Content -LiteralPath (Join-Path $Output 'reconciliation-pass.json') -Raw | ConvertFrom-Json
    if (-not $Pass.passed -or $Pass.matched -ne 3 -or $Pass.missing -ne 0) { throw 'passing reconciliation report is incorrect' }

    $FailReport = "$ContainerOutput/reconciliation-fail.json"
    & docker run --rm @Mounts rabbit-jetstream/operator:migration-test migrate reconcile --source tests/fixtures/migration-shadow-source.ndjson --target tests/fixtures/migration-shadow-drift.ndjson --output $FailReport 2>$null
    if ($LASTEXITCODE -eq 0) { throw 'drifted message evidence unexpectedly passed' }
    $Fail = Get-Content -LiteralPath (Join-Path $Output 'reconciliation-fail.json') -Raw | ConvertFrom-Json
    if ($Fail.passed -or $Fail.missing -ne 1 -or $Fail.unexpected -ne 1 -or $Fail.content_mismatch -ne 1 -or $Fail.target_duplicates -ne 1) { throw 'failed reconciliation report is incorrect' }
} finally {
    Remove-Item -LiteralPath $TemporaryRoot -Recurse -Force -ErrorAction SilentlyContinue
    Pop-Location
}
