$ErrorActionPreference = 'Stop'
$RepositoryRoot = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
$Output = Join-Path $RepositoryRoot ".tmp-rjs-migration-$PID"
$RootPrefix = [IO.Path]::GetFullPath($RepositoryRoot).TrimEnd([IO.Path]::DirectorySeparatorChar) + [IO.Path]::DirectorySeparatorChar
if (-not [IO.Path]::GetFullPath($Output).StartsWith($RootPrefix, [StringComparison]::OrdinalIgnoreCase)) { throw 'temporary migration path escaped the repository' }

function Invoke-Docker {
    & docker @args
    if ($LASTEXITCODE -ne 0) { throw "docker command failed: docker $args" }
}

Push-Location $RepositoryRoot
try {
    Invoke-Docker build -f packaging/Dockerfile.operator -t rabbit-jetstream/operator:migration-test .
    $ContainerOutput = "/src/$([IO.Path]::GetFileName($Output))"
    Invoke-Docker run --rm -v "${RepositoryRoot}:/src" -w /src rabbit-jetstream/operator:migration-test migrate rabbitmq-definitions --output $ContainerOutput tests/fixtures/rabbitmq-definitions.json
    $Report = Get-Content -LiteralPath (Join-Path $Output 'migration-report.json') -Raw | ConvertFrom-Json
    if (-not $Report.compatible -or $Report.converted -ne 3 -or $Report.queues -ne 3) { throw 'migration report is incorrect' }
    foreach ($Name in @('orders', 'orders_dlq', 'audit')) {
        Invoke-Docker run --rm -v "${RepositoryRoot}:/src:ro" -w /src rabbit-jetstream/operator:migration-test queue validate "$ContainerOutput/$Name.yaml"
    }
} finally {
    Remove-Item -LiteralPath $Output -Recurse -Force -ErrorAction SilentlyContinue
    Pop-Location
}
