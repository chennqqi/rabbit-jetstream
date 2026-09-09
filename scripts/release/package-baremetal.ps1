param([string]$RuntimeBundle = 'dist/v0.1.0-rc.2')
$ErrorActionPreference = 'Stop'
$Repo = (Resolve-Path (Join-Path $PSScriptRoot '../..')).Path
$Frozen = (Resolve-Path (Join-Path $Repo $RuntimeBundle)).Path
function Checked([string]$Command, [string[]]$Arguments) {
    & $Command @Arguments
    if ($LASTEXITCODE -ne 0) { throw "$Command failed" }
}
function JsonWrite([string]$Path, $Value) {
    [IO.File]::WriteAllText($Path, ($Value | ConvertTo-Json -Depth 20) + "`n", [Text.UTF8Encoding]::new($false))
}
function Digest([string]$Path) { (Get-FileHash -Algorithm SHA256 -LiteralPath $Path).Hash.ToLowerInvariant() }
$Verifier = (& git -C $Repo rev-parse HEAD).Trim()
if ($LASTEXITCODE -ne 0 -or $Verifier -notmatch '^[a-f0-9]{40}$') { throw 'invalid verification revision' }
if (& git -C $Repo status --porcelain) { throw 'freeze verification tools in a clean worktree before packaging' }
$Manifest = Get-Content -Raw -LiteralPath (Join-Path $Frozen 'release-manifest.json') | ConvertFrom-Json
foreach ($Line in Get-Content -LiteralPath (Join-Path $Frozen 'SHA256SUMS')) {
    if ($Line -notmatch '^([a-f0-9]{64})  ([A-Za-z0-9_./-]+)$') { throw 'invalid frozen checksum entry' }
    $Expected = $Matches[1]; $Relative = $Matches[2]
    if ($Relative.StartsWith('/') -or $Relative.Split('/') -contains '..') { throw 'unsafe checksum path' }
    if ((Digest (Join-Path $Frozen $Relative)) -ne $Expected) { throw "frozen checksum mismatch: $Relative" }
}
# Runtime binaries remain byte-identical to the previously qualified release.
# Only separately versioned qualification tooling is built from current HEAD.
$Destination = Join-Path $Repo "dist/$($Manifest.version)-baremetal-$($Verifier.Substring(0,12))"
if (Test-Path -LiteralPath $Destination) { throw 'output already exists' }
New-Item -ItemType Directory -Path $Destination | Out-Null
New-Item -ItemType Directory -Path (Join-Path $Destination 'bin/linux-amd64'), (Join-Path $Destination 'scripts'), (Join-Path $Destination 'licenses') | Out-Null
foreach ($Name in @('rjs-management','rjsctl')) { Copy-Item -LiteralPath (Join-Path $Frozen "bin/linux-amd64/$Name") -Destination (Join-Path $Destination "bin/linux-amd64/$Name") }
Copy-Item -Path (Join-Path $Frozen 'licenses/*') -Destination (Join-Path $Destination 'licenses')
Copy-Item -LiteralPath (Join-Path $Frozen 'release-manifest.json') -Destination (Join-Path $Destination 'runtime-manifest.json')
# Extract the binary locally from the checksum-verified frozen OCI archive.
# No container needs to be started to extract it; no OCI archive goes to the host.
$Archive = Join-Path $Frozen 'images/nats.oci.tar'
$Scratch = Join-Path $Repo "artifacts/baremetal-extract-$($Verifier.Substring(0,12))"
if (Test-Path -LiteralPath $Scratch) { throw 'extraction directory already exists' }
New-Item -ItemType Directory -Path $Scratch | Out-Null
function ReadArchiveJSON([string]$Entry) {
    $Raw = & tar -xOf $Archive $Entry
    if ($LASTEXITCODE -ne 0) { throw "cannot read OCI entry $Entry" }
    ($Raw -join "`n") | ConvertFrom-Json
}
$Index = ReadArchiveJSON 'index.json'
while ($Index.manifests) {
    $Choices = @($Index.manifests | Where-Object { $_.platform.os -eq 'linux' -and $_.platform.architecture -eq 'amd64' })
    if ($Choices.Count -eq 1) { $Descriptor = $Choices[0] }
    elseif ($Index.manifests.Count -eq 1 -and -not $Index.manifests[0].platform) { $Descriptor = $Index.manifests[0] }
    else { throw 'ambiguous AMD64 OCI descriptor' }
    if ($Descriptor.digest -notmatch '^sha256:[a-f0-9]{64}$') { throw 'invalid OCI digest' }
    $PlatformDigest = $Descriptor.digest
    $Index = ReadArchiveJSON ('blobs/sha256/' + $Descriptor.digest.Substring(7))
}
$ConfigDigest = $Index.config.digest
$Found = $false
foreach ($Layer in $Index.layers) {
    if ($Layer.digest -notmatch '^sha256:[a-f0-9]{64}$') { throw 'invalid layer digest' }
    $Entry = 'blobs/sha256/' + $Layer.digest.Substring(7)
    Checked 'tar' @('-xf', $Archive, '-C', $Scratch, $Entry)
    $LayerPath = Join-Path $Scratch $Entry
    if ((Digest $LayerPath) -ne $Layer.digest.Substring(7)) { throw 'OCI layer checksum mismatch' }
    $Entries = & tar -tf $LayerPath
    if ($LASTEXITCODE -ne 0) { throw 'cannot list layer' }
    if ($Entries -contains 'usr/local/bin/nats-server') {
        Checked 'tar' @('-xf', $LayerPath, '-C', $Scratch, 'usr/local/bin/nats-server')
        Copy-Item -LiteralPath (Join-Path $Scratch 'usr/local/bin/nats-server') -Destination (Join-Path $Destination 'bin/linux-amd64/nats-server')
        $Found = $true
    }
}
if (-not $Found) { throw 'NATS executable absent from frozen OCI layers' }
$GoImage = 'golang:1.25.13-alpine@sha256:1e0126852075c9c60731c8ba49088448b91f63e2aed97ca9d1a9791622a05946'
$ModuleCache = (& go env GOMODCACHE).Trim()
$Build = 'set -eu; for name in nativequal perfevidence resourceaudit baremetal-run; do go build -buildvcs=false -trimpath -ldflags=-s -o /out/bin/linux-amd64/$name ./tools/$name; done; for name in jetstream-bench resource-sampler; do go build -buildvcs=false -trimpath -ldflags=-s -o /out/bin/linux-amd64/$name ./tests/helpers/$name; done'
Checked 'docker' @('run','--rm','--network=none','-e','CGO_ENABLED=0','-e','GOOS=linux','-e','GOARCH=amd64','-e','GOPROXY=off','-v',"${Repo}:/src:ro",'-v',"${Destination}:/out",'-v',"${ModuleCache}:/go/pkg/mod",'-w','/src',$GoImage,'sh','-c',$Build)
# Normalize the deployable shell helper to LF, without changing repository files.
$Launcher = [IO.File]::ReadAllText((Join-Path $Repo 'scripts/release/baremetal-systemd.sh')).Replace("`r`n","`n")
[IO.File]::WriteAllText((Join-Path $Destination 'scripts/baremetal-systemd.sh'),$Launcher,[Text.UTF8Encoding]::new($false))
$Artifacts = @(Get-ChildItem -LiteralPath $Destination -File -Recurse | Sort-Object FullName | ForEach-Object {
    [ordered]@{ path=$_.FullName.Substring($Destination.Length+1).Replace('\','/'); bytes=$_.Length; sha256=(Digest $_.FullName) }
})
$BareManifest = [ordered]@{
    schema='rabbit-jetstream.io/release-bundle/v1alpha1'; deployment_mode='bare-metal'; version=$Manifest.version
    server_revision=$Manifest.server_revision; verification_revision=$Verifier; sdk=$Manifest.sdk; contract_version=$Manifest.contract_version
    nats_version=$Manifest.nats_version; platforms=@('linux/amd64'); generated_at=[DateTime]::UtcNow.ToString('o')
    qualification='native-linux-soak-and-canary-required'; runtime_manifest_sha256=(Digest (Join-Path $Frozen 'release-manifest.json'))
    nats_oci_platform_digest=$PlatformDigest; nats_oci_config_digest=$ConfigDigest; artifacts=$Artifacts
}
JsonWrite (Join-Path $Destination 'release-manifest.json') $BareManifest
$Sums = @($Artifacts | ForEach-Object { "$($_.sha256)  $($_.path)" }) + @("$(Digest (Join-Path $Destination 'release-manifest.json'))  release-manifest.json")
[IO.File]::WriteAllText((Join-Path $Destination 'SHA256SUMS'),($Sums -join "`n")+"`n",[Text.UTF8Encoding]::new($false))
Checked 'tar' @('-czf',"$Destination.tar.gz",'-C',$Destination,'.')
Write-Output "Bare-metal bundle: $Destination.tar.gz"
Write-Output "SHA256: $(Digest "$Destination.tar.gz")"
