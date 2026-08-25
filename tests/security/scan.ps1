$ErrorActionPreference = 'Stop'
$RepositoryRoot = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
$Trivy = 'aquasec/trivy@sha256:be1190afcb28352bfddc4ddeb71470835d16462af68d310f9f4bca710961a41e'
$Cache = 'rjs-trivy-' + ([guid]::NewGuid().ToString('N')).Substring(0, 12)
$Images = @(
    @{Dockerfile = 'packaging/Dockerfile.nats-server'; Tag = 'rabbit-jetstream/nats-server:security-test'},
    @{Dockerfile = 'packaging/Dockerfile.management'; Tag = 'rabbit-jetstream/management:security-test'},
    @{Dockerfile = 'packaging/Dockerfile.operator'; Tag = 'rabbit-jetstream/operator:security-test'}
)

function Invoke-Checked {
    & $args[0] $args[1..($args.Count - 1)]
    if ($LASTEXITCODE -ne 0) { throw "command failed: $($args -join ' ')" }
}

function Invoke-TrivyScan([string]$Image) {
    for ($Attempt = 1; $Attempt -le 3; $Attempt++) {
        & docker run --rm -v /var/run/docker.sock:/var/run/docker.sock -v "${Cache}:/root/.cache/trivy" $Trivy image --skip-version-check --scanners vuln --ignore-unfixed --severity 'HIGH,CRITICAL' --exit-code 1 $Image
        if ($LASTEXITCODE -eq 0) { return }
        if ($Attempt -lt 3) { Start-Sleep -Seconds 2 }
    }
    throw "Trivy scan failed after 3 attempts: $Image"
}

Push-Location $RepositoryRoot
try {
    Invoke-Checked go run golang.org/x/vuln/cmd/govulncheck@v1.6.0 ./...
    foreach ($Image in $Images) {
        Invoke-Checked docker build -f $Image.Dockerfile -t $Image.Tag .
    }
    Invoke-Checked docker volume create $Cache
    foreach ($Image in $Images) {
        Invoke-TrivyScan $Image.Tag
    }
} finally {
    & docker volume rm $Cache 2>$null | Out-Null
    Pop-Location
}
