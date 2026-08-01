$ErrorActionPreference = 'Stop'
$RepositoryRoot = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
$Chart = 'deploy/helm/rabbit-jetstream'
$Temporary = Join-Path ([IO.Path]::GetTempPath()) ("rjs-helm-{0}" -f [guid]::NewGuid())
New-Item -ItemType Directory -Path $Temporary | Out-Null

function Invoke-Docker {
    & docker @args
    if ($LASTEXITCODE -ne 0) { throw "docker command failed: docker $args" }
}

function Build-DockerImage([string]$Dockerfile, [string]$Tag) {
	foreach ($Attempt in 1..3) {
		& docker build -f $Dockerfile -t $Tag .
		if ($LASTEXITCODE -eq 0) { return }
		if ($Attempt -lt 3) { Start-Sleep -Seconds (2 * $Attempt) }
	}
	throw "failed to build $Tag after 3 attempts"
}

Push-Location $RepositoryRoot
try {
    Invoke-Docker run --rm -v "${RepositoryRoot}:/src:ro" -w /src alpine/helm:3.18.4 lint $Chart

    $Default = Join-Path $Temporary 'default.yaml'
    & docker run --rm -v "${RepositoryRoot}:/src:ro" -w /src alpine/helm:3.18.4 template production $Chart --namespace messaging | Set-Content -LiteralPath $Default -Encoding utf8NoBOM
    if ($LASTEXITCODE -ne 0) { throw 'default Helm render failed' }
    Invoke-Docker run --rm -v "${Temporary}:/work:ro" ghcr.io/yannh/kubeconform:v0.6.7 -strict -summary /work/default.yaml

    $Rendered = Get-Content -LiteralPath $Default -Raw
    foreach ($Required in @('kind: StatefulSet', 'replicas: 3', 'kind: PodDisruptionBudget', 'kind: NetworkPolicy', 'runAsNonRoot: true', 'RJS_METADATA_REPLICAS', 'RJS_ADMIN_TOKENS', 'RJS_AUDIT_TOKENS', 'optional: true')) {
        if (-not $Rendered.Contains($Required)) { throw "default Helm output is missing $Required" }
    }
	function Read-SecretValue([string]$Key) {
		$Pattern = '(?m)^  ' + [regex]::Escape($Key) + ':\s+["'']?([^"''\r\n]+)'
		$Match = [regex]::Match($Rendered, $Pattern)
		if (-not $Match.Success) { throw "generated Secret is missing $Key" }
		return [Text.Encoding]::UTF8.GetString([Convert]::FromBase64String($Match.Groups[1].Value))
	}
	$NATSUsername = Read-SecretValue 'nats-username'
	$NATSPassword = Read-SecretValue 'nats-password'
	$NATSPasswordHash = Read-SecretValue 'nats-password-bcrypt'
	$ConfigMatch = [regex]::Match($Rendered, '(?m)^  nats\.conf: \|\r?\n(?<config>(?:    [^\r\n]*(?:\r?\n|$))+)(?=---|\z)')
	if (-not $ConfigMatch.Success) { throw 'rendered NATS configuration was not found' }
	$NATSConfig = [regex]::Replace($ConfigMatch.Groups['config'].Value, '(?m)^    ', '')
	$NATSConfigPath = Join-Path $Temporary 'nats.conf'
	Set-Content -LiteralPath $NATSConfigPath -Value $NATSConfig -Encoding utf8NoBOM
	Build-DockerImage 'packaging/Dockerfile.nats-server' 'rabbit-jetstream/nats-server:helm-test'
	Invoke-Docker run --rm -e POD_NAME=rjs-test-0 -e "RJS_NATS_USER=$NATSUsername" -e "RJS_NATS_PASSWORD_HASH=$NATSPasswordHash" -v "${Temporary}:/work:ro" rabbit-jetstream/nats-server:helm-test -t -c /work/nats.conf

	$SingleRendered = (& docker run --rm -v "${RepositoryRoot}:/src:ro" -w /src alpine/helm:3.18.4 template production $Chart --namespace messaging --set nats.replicaCount=1) -join "`n"
	if ($LASTEXITCODE -ne 0) { throw 'single-node Helm render failed' }
	$SingleMatch = [regex]::Match($SingleRendered, '(?m)^  nats\.conf: \|\r?\n(?<config>(?:    [^\r\n]*(?:\r?\n|$))+)(?=---|\z)')
	if (-not $SingleMatch.Success) { throw 'single-node NATS configuration was not found' }
	$SingleConfig = [regex]::Replace($SingleMatch.Groups['config'].Value, '(?m)^    ', '')
	Set-Content -LiteralPath (Join-Path $Temporary 'nats-single.conf') -Value $SingleConfig -Encoding utf8NoBOM
	Build-DockerImage 'packaging/Dockerfile.management' 'rabbit-jetstream/management:helm-test'
	$RuntimeID = ([guid]::NewGuid().ToString('N')).Substring(0, 12)
	$Network = "rjs-helm-$RuntimeID"
	$NATSContainer = "rjs-helm-nats-$RuntimeID"
	$ManagementContainer = "rjs-helm-management-$RuntimeID"
	Invoke-Docker network create $Network
	try {
		Invoke-Docker run -d --name $NATSContainer --network $Network --network-alias production-rabbit-jetstream-nats --network-alias production-rabbit-jetstream-nats-0.production-rabbit-jetstream-nats-headless -e POD_NAME=production-rabbit-jetstream-nats-0 -e "RJS_NATS_USER=$NATSUsername" -e "RJS_NATS_PASSWORD_HASH=$NATSPasswordHash" -v "${Temporary}:/work:ro" rabbit-jetstream/nats-server:helm-test -c /work/nats-single.conf
		Invoke-Docker run -d --name $ManagementContainer --network $Network --network-alias management -e RJS_NATS_URL=nats://production-rabbit-jetstream-nats:4222 -e RJS_NATS_MONITOR_URLS=http://production-rabbit-jetstream-nats-0.production-rabbit-jetstream-nats-headless:8222 -e "RJS_NATS_USER=$NATSUsername" -e "RJS_NATS_PASSWORD=$NATSPassword" -e RJS_ADMIN_TOKEN=test-admin-token -e RJS_METADATA_REPLICAS=1 -e RJS_INSTANCE_ID=helm-runtime-test rabbit-jetstream/management:helm-test
		$Ready = $false
		foreach ($Attempt in 1..30) {
			& docker run --rm --network $Network busybox:1.37.0 wget -q -O - http://management:8223/readyz 2>$null | Out-Null
			if ($LASTEXITCODE -eq 0) { $Ready = $true; break }
			Start-Sleep -Seconds 1
		}
		if (-not $Ready) {
			& docker logs $NATSContainer
			& docker logs $ManagementContainer
			throw 'rendered Helm configuration did not produce a ready management service'
		}
	} finally {
		& docker rm -f -v $ManagementContainer $NATSContainer 2>$null | Out-Null
		& docker network rm $Network 2>$null | Out-Null
	}

	$ClusterID = ([guid]::NewGuid().ToString('N')).Substring(0, 12)
	$ClusterNetwork = "rjs-helm-cluster-$ClusterID"
	$ClusterManagement = "rjs-helm-cluster-management-$ClusterID"
	$ClusterNodes = @(0..2 | ForEach-Object { "rjs-helm-cluster-nats-$ClusterID-$_" })
	Invoke-Docker network create $ClusterNetwork
	try {
		foreach ($Index in 0..2) {
			$FQDN = "production-rabbit-jetstream-nats-$Index.production-rabbit-jetstream-nats-headless"
			Invoke-Docker run -d --name $ClusterNodes[$Index] --network $ClusterNetwork --network-alias $FQDN -e "POD_NAME=production-rabbit-jetstream-nats-$Index" -e "RJS_NATS_USER=$NATSUsername" -e "RJS_NATS_PASSWORD_HASH=$NATSPasswordHash" -v "${Temporary}:/work:ro" rabbit-jetstream/nats-server:helm-test -c /work/nats.conf
		}
		Invoke-Docker run -d --name $ClusterManagement --network $ClusterNetwork --network-alias cluster-management -e 'RJS_NATS_URL=nats://production-rabbit-jetstream-nats-0.production-rabbit-jetstream-nats-headless:4222,nats://production-rabbit-jetstream-nats-1.production-rabbit-jetstream-nats-headless:4222,nats://production-rabbit-jetstream-nats-2.production-rabbit-jetstream-nats-headless:4222' -e 'RJS_NATS_MONITOR_URLS=http://production-rabbit-jetstream-nats-0.production-rabbit-jetstream-nats-headless:8222,http://production-rabbit-jetstream-nats-1.production-rabbit-jetstream-nats-headless:8222,http://production-rabbit-jetstream-nats-2.production-rabbit-jetstream-nats-headless:8222' -e "RJS_NATS_USER=$NATSUsername" -e "RJS_NATS_PASSWORD=$NATSPassword" -e RJS_ADMIN_TOKEN=test-admin-token -e RJS_METADATA_REPLICAS=3 -e RJS_CONTROLLER_INTERVAL=1s -e RJS_CONTROLLER_LEASE_TTL=4s -e RJS_INSTANCE_ID=helm-cluster-test rabbit-jetstream/management:helm-test
		$Leader = $false
		foreach ($Attempt in 1..45) {
			$Status = & docker run --rm --network $ClusterNetwork busybox:1.37.0 wget -q -O - http://cluster-management:8223/api/v1/controller 2>$null
			if ($LASTEXITCODE -eq 0 -and ($Status -join '') -match '"leader":true') { $Leader = $true; break }
			Start-Sleep -Seconds 1
		}
		if (-not $Leader) {
			foreach ($Node in $ClusterNodes) { & docker logs $Node }
			& docker logs $ClusterManagement
			throw 'three-node rendered Helm cluster did not elect a controller with replicated metadata'
		}
	} finally {
		& docker rm -f -v $ClusterManagement @ClusterNodes 2>$null | Out-Null
		& docker network rm $ClusterNetwork 2>$null | Out-Null
	}

    $Optional = Join-Path $Temporary 'optional.yaml'
    & docker run --rm -v "${RepositoryRoot}:/src:ro" -w /src alpine/helm:3.18.4 template production $Chart --namespace messaging --set serviceMonitor.enabled=true --set ingress.enabled=true | Set-Content -LiteralPath $Optional -Encoding utf8NoBOM
    if ($LASTEXITCODE -ne 0) { throw 'optional Helm render failed' }
    Invoke-Docker run --rm -v "${Temporary}:/work:ro" ghcr.io/yannh/kubeconform:v0.6.7 -strict -ignore-missing-schemas -summary /work/optional.yaml

    $ExistingSecret = & docker run --rm -v "${RepositoryRoot}:/src:ro" -w /src alpine/helm:3.18.4 template production $Chart --namespace messaging --set auth.existingSecret=production-auth
    if ($LASTEXITCODE -ne 0 -or ($ExistingSecret -join "`n") -match '(?m)^kind: Secret$') { throw 'existing Secret mode rendered a generated Secret' }

    $TLSRendered = (& docker run --rm -v "${RepositoryRoot}:/src:ro" -w /src alpine/helm:3.18.4 template production $Chart --namespace messaging --set nats.tls.enabled=true --set nats.tls.serverSecret=production-nats-server-tls --set nats.tls.clientSecret=production-nats-client-tls) -join "`n"
    if ($LASTEXITCODE -ne 0) { throw 'mTLS Helm render failed' }
    foreach ($Required in @('tls://production-rabbit-jetstream-nats:4222', 'RJS_NATS_TLS_CA', 'RJS_NATS_TLS_CERT', 'RJS_NATS_TLS_KEY', 'verify: true', 'secretName: production-nats-server-tls', 'secretName: production-nats-client-tls')) {
        if (-not $TLSRendered.Contains($Required)) { throw "mTLS Helm output is missing $Required" }
    }
	$TLSDirectory = Join-Path $Temporary 'tls'
	& go run ./tests/helpers/tls-fixture --output $TLSDirectory
	if ($LASTEXITCODE -ne 0) { throw 'test TLS certificate generation failed' }
	$TLSSingleRendered = (& docker run --rm -v "${RepositoryRoot}:/src:ro" -w /src alpine/helm:3.18.4 template production $Chart --namespace messaging --set nats.replicaCount=1 --set nats.tls.enabled=true --set nats.tls.serverSecret=production-nats-server-tls --set nats.tls.clientSecret=production-nats-client-tls) -join "`n"
	$TLSConfigMatch = [regex]::Match($TLSSingleRendered, '(?m)^  nats\.conf: \|\r?\n(?<config>(?:    [^\r\n]*(?:\r?\n|$))+)(?=---|\z)')
	if (-not $TLSConfigMatch.Success) { throw 'mTLS NATS configuration was not found' }
	$TLSConfig = [regex]::Replace($TLSConfigMatch.Groups['config'].Value, '(?m)^    ', '')
	Set-Content -LiteralPath (Join-Path $Temporary 'nats-tls.conf') -Value $TLSConfig -Encoding utf8NoBOM
	$TLSID = ([guid]::NewGuid().ToString('N')).Substring(0, 12)
	$TLSNetwork = "rjs-helm-tls-$TLSID"
	$TLSNATS = "rjs-helm-tls-nats-$TLSID"
	$TLSManagement = "rjs-helm-tls-management-$TLSID"
	Invoke-Docker network create $TLSNetwork
	try {
		Invoke-Docker run -d --name $TLSNATS --network $TLSNetwork --network-alias production-rabbit-jetstream-nats -e POD_NAME=production-rabbit-jetstream-nats-0 -e "RJS_NATS_USER=$NATSUsername" -e "RJS_NATS_PASSWORD_HASH=$NATSPasswordHash" -v "${Temporary}:/work:ro" -v "${TLSDirectory}:/etc/nats-tls:ro" rabbit-jetstream/nats-server:helm-test -c /work/nats-tls.conf
		Invoke-Docker run -d --name $TLSManagement --network $TLSNetwork --network-alias tls-management -e RJS_NATS_URL=tls://production-rabbit-jetstream-nats:4222 -e RJS_NATS_MONITOR_URLS=http://production-rabbit-jetstream-nats:8222 -e "RJS_NATS_USER=$NATSUsername" -e "RJS_NATS_PASSWORD=$NATSPassword" -e RJS_NATS_TLS_CA=/etc/nats-tls/ca.crt -e RJS_NATS_TLS_CERT=/etc/nats-tls/tls.crt -e RJS_NATS_TLS_KEY=/etc/nats-tls/tls.key -e RJS_NATS_TLS_SERVER_NAME=production-rabbit-jetstream-nats -e RJS_ADMIN_TOKEN=test-admin-token -e RJS_METADATA_REPLICAS=1 -v "${TLSDirectory}:/etc/nats-tls:ro" rabbit-jetstream/management:helm-test
		$TLSReady = $false
		foreach ($Attempt in 1..30) {
			& docker run --rm --network $TLSNetwork busybox:1.37.0 wget -q -O - http://tls-management:8223/readyz 2>$null | Out-Null
			if ($LASTEXITCODE -eq 0) { $TLSReady = $true; break }
			Start-Sleep -Seconds 1
		}
		if (-not $TLSReady) {
			& docker logs $TLSNATS
			& docker logs $TLSManagement
			throw 'rendered mTLS profile did not produce a ready management service'
		}
	} finally {
		& docker rm -f -v $TLSManagement $TLSNATS 2>$null | Out-Null
		& docker network rm $TLSNetwork 2>$null | Out-Null
	}
    & docker run --rm -v "${RepositoryRoot}:/src:ro" -w /src alpine/helm:3.18.4 template invalid $Chart --set nats.tls.enabled=true 2>$null | Out-Null
    if ($LASTEXITCODE -eq 0) { throw 'mTLS Helm render accepted a missing certificate Secret' }

    & docker run --rm -v "${RepositoryRoot}:/src:ro" -w /src alpine/helm:3.18.4 template invalid $Chart --set nats.replicaCount=2 2>$null | Out-Null
    if ($LASTEXITCODE -eq 0) { throw 'values schema accepted an even NATS replica count' }
} finally {
    Pop-Location
    Remove-Item -LiteralPath $Temporary -Recurse -Force -ErrorAction SilentlyContinue
}
