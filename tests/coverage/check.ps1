param(
    [double]$Minimum = $(if ($env:RJS_COVERAGE_MIN) { [double]$env:RJS_COVERAGE_MIN } else { 80.0 })
)

$ErrorActionPreference = 'Stop'
$profile = Join-Path ([System.IO.Path]::GetTempPath()) ("rjs-coverage-{0}.out" -f [guid]::NewGuid())
try {
    go test "-coverprofile=$profile" ./...
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
    $total = go tool cover "-func=$profile" | Select-String '^total:'
    if ($LASTEXITCODE -ne 0 -or -not $total) { throw 'unable to read total coverage' }
    $match = [regex]::Match($total.Line, '([0-9]+(?:\.[0-9]+)?)%')
    if (-not $match.Success) { throw "unable to parse coverage: $($total.Line)" }
    $actual = [double]::Parse($match.Groups[1].Value, [Globalization.CultureInfo]::InvariantCulture)
    Write-Host ("total coverage: {0:N1}% (required: {1:N1}%)" -f $actual, $Minimum)
    if ($actual -lt $Minimum) { throw "coverage $actual% is below $Minimum%" }
}
finally {
    Remove-Item -LiteralPath $profile -Force -ErrorAction SilentlyContinue
}
