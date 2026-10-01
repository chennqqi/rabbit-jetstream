param(
    [switch]$Check
)

$ErrorActionPreference = 'Stop'
$repo = Split-Path -Parent $PSScriptRoot
$temporary = Join-Path ([IO.Path]::GetTempPath()) ("rjs-openapi-codegen-" + [Guid]::NewGuid().ToString('N'))
$bundle = Join-Path $temporary 'openapi.codegen.yaml'
$goOutput = if ($Check) { Join-Path $temporary 'models.gen.go' } else { Join-Path $repo 'management/internal/apigen/models.gen.go' }
$tsOutput = if ($Check) { Join-Path $temporary 'openapi.d.ts' } else { Join-Path $repo 'admin-ui/src/generated/openapi.d.ts' }
$goExpected = Join-Path $repo 'management/internal/apigen/models.gen.go'
$tsExpected = Join-Path $repo 'admin-ui/src/generated/openapi.d.ts'
$tsGenerator = Join-Path $repo 'admin-ui/scripts/generate-openapi-types.mjs'

try {
    New-Item -ItemType Directory -Path $temporary -Force | Out-Null
    New-Item -ItemType Directory -Path (Split-Path -Parent $goOutput),(Split-Path -Parent $tsOutput) -Force | Out-Null

    & go run ./tools/openapibundle -out $bundle
    if ($LASTEXITCODE -ne 0) { throw 'OpenAPI bundling failed' }
    & go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.8.0 -generate types -package apigen -o $goOutput $bundle
    if ($LASTEXITCODE -ne 0) { throw 'Go OpenAPI generation failed' }
    & node $tsGenerator $bundle $tsOutput
    if ($LASTEXITCODE -ne 0) { throw 'TypeScript OpenAPI generation failed' }

    if ($Check) {
        foreach ($pair in @(@($goExpected, $goOutput), @($tsExpected, $tsOutput))) {
            if (!(Test-Path -LiteralPath $pair[0]) -or !(Test-Path -LiteralPath $pair[1])) {
                throw "Generated file is missing: $($pair[0])"
            }
            $expected = [IO.File]::ReadAllBytes($pair[0])
            $actual = [IO.File]::ReadAllBytes($pair[1])
            if ($expected.Length -ne $actual.Length -or [Convert]::ToHexString([Security.Cryptography.SHA256]::HashData($expected)) -ne [Convert]::ToHexString([Security.Cryptography.SHA256]::HashData($actual))) {
                throw "Generated OpenAPI file is stale: $($pair[0]). Run scripts/openapi-codegen.ps1"
            }
        }
    }
}
finally {
    if (Test-Path -LiteralPath $temporary) {
        Remove-Item -LiteralPath $temporary -Recurse -Force
    }
}
