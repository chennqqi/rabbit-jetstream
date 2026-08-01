param(
    [ValidateSet('standalone', 'api', 'reconcile', 'apply', 'delete', 'routing', 'dlq', 'controller', 'fault')]
    [string]$Scenario = 'standalone'
)

$ErrorActionPreference = 'Stop'
$RepositoryRoot = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path

function Invoke-Docker {
    & docker @args
    if ($LASTEXITCODE -ne 0) {
        throw "docker command failed: docker $args"
    }
}

function Wait-Healthy([string]$ContainerName) {
    foreach ($Attempt in 1..45) {
        $State = & docker inspect --format '{{.State.Health.Status}}' $ContainerName 2>$null
        if ($LASTEXITCODE -eq 0 -and $State -eq 'healthy') { return }
        Start-Sleep -Seconds 1
    }
    throw "$ContainerName did not become healthy"
}

function Get-NetworkJson([string]$Network, [string]$Uri) {
    $Value = & docker run --rm --network $Network alpine:3.23 wget -q -O - $Uri | ConvertFrom-Json
    if ($LASTEXITCODE -ne 0) { throw "failed to query $Uri on $Network" }
    return $Value
}

function Wait-NetworkControllerLeader([string]$Network, [string]$Uri) {
    foreach ($Attempt in 1..30) {
        try {
            $Status = Get-NetworkJson $Network $Uri
            if ($Status.leader) { return $Status }
        } catch {}
        Start-Sleep -Seconds 1
    }
    throw "controller at $Uri did not become leader"
}

Push-Location $RepositoryRoot
try {
    if ($Scenario -in @('standalone', 'api', 'reconcile', 'apply', 'delete', 'routing', 'dlq')) {
        $Project = "rjs-desktop-$Scenario"
        $Compose = 'deploy/compose/standalone.yml'
		$PreviousAdminToken = $env:RJS_ADMIN_TOKEN
		if ($Scenario -in @('apply', 'delete', 'routing', 'dlq')) { $env:RJS_ADMIN_TOKEN = 'desktop-test-token' }
        try {
            Invoke-Docker compose -p $Project -f $Compose up -d --build --wait
            $Ready = Invoke-RestMethod -Uri 'http://127.0.0.1:8223/readyz' -TimeoutSec 5
            if ($Ready.status -ne 'ready') { throw 'management API is not ready' }
            if ($Scenario -eq 'api') {
                $Network = "${Project}_default"
                Invoke-Docker run --rm --network $Network natsio/nats-box:latest nats --server nats://nats:4222 stream add RJS_API --subjects rjs.api --storage file --replicas 1 --defaults
                Invoke-Docker run --rm --network $Network natsio/nats-box:latest nats --server nats://nats:4222 consumer add RJS_API WORKER --filter rjs.api --ack explicit --pull --defaults
                $Cluster = Invoke-RestMethod -Uri 'http://127.0.0.1:8223/api/v1/cluster' -TimeoutSec 5
                $Streams = Invoke-RestMethod -Uri 'http://127.0.0.1:8223/api/v1/streams' -TimeoutSec 5
                $Stream = Invoke-RestMethod -Uri 'http://127.0.0.1:8223/api/v1/streams/RJS_API' -TimeoutSec 5
                $Consumers = Invoke-RestMethod -Uri 'http://127.0.0.1:8223/api/v1/streams/RJS_API/consumers' -TimeoutSec 5
                $Nodes = Invoke-RestMethod -Uri 'http://127.0.0.1:8223/api/v1/nodes' -TimeoutSec 5
                if ($Cluster.account.streams -ne 1) { throw 'cluster stream count is incorrect' }
                if ($Streams.total -ne 1 -or $Streams.items[0].name -ne 'RJS_API') { throw 'stream list is incorrect' }
                if ($Stream.replicas -ne 1) { throw 'stream detail is incorrect' }
                if ($Consumers.total -ne 1 -or $Consumers.items[0].name -ne 'WORKER') { throw 'consumer list is incorrect' }
                if ($Nodes.status -ne 'available' -or $Nodes.available -ne 1) { throw 'standalone node monitoring is incorrect' }
            }
            if ($Scenario -eq 'reconcile') {
                $Network = "${Project}_default"
                $Result = & docker run --rm --network $Network -v "${RepositoryRoot}:/src" -w /src golang:1.25-bookworm go run ./tools/rjsctl queue reconcile --url http://management:8223 tests/fixtures/queue-basic.yaml | ConvertFrom-Json
                if ($LASTEXITCODE -ne 0) { throw 'Linux rjsctl reconcile failed' }
                if ($Result.status -ne 'ready' -or $Result.blocked) { throw 'reconcile did not produce a ready plan' }
                if (@($Result.operations | Where-Object action -eq 'create').Count -ne 2) { throw 'reconcile did not plan two creates' }
                $Streams = Invoke-RestMethod -Uri 'http://127.0.0.1:8223/api/v1/streams' -TimeoutSec 5
                if ($Streams.total -ne 0) { throw 'reconcile unexpectedly wrote JetStream resources' }
            }
			if ($Scenario -eq 'apply') {
				$Network = "${Project}_default"
				$First = & docker run --rm --network $Network -e RJS_ADMIN_TOKEN=desktop-test-token -v "${RepositoryRoot}:/src" -w /src golang:1.25-bookworm go run ./tools/rjsctl queue apply --url http://management:8223 tests/fixtures/queue-basic.yaml | ConvertFrom-Json
				if ($LASTEXITCODE -ne 0 -or $First.status -ne 'ready') { throw 'first apply failed' }
				$Second = & docker run --rm --network $Network -e RJS_ADMIN_TOKEN=desktop-test-token -v "${RepositoryRoot}:/src" -w /src golang:1.25-bookworm go run ./tools/rjsctl queue apply --url http://management:8223 tests/fixtures/queue-basic.yaml | ConvertFrom-Json
				if ($LASTEXITCODE -ne 0 -or $Second.status -ne 'noop') { throw 'second apply was not idempotent' }
				$OldETag = [string]((Invoke-WebRequest -Uri 'http://127.0.0.1:8223/api/v1/queues/basic' -TimeoutSec 5).Headers.ETag | Select-Object -First 1)
				$Updated = & docker run --rm --network $Network -e RJS_ADMIN_TOKEN=desktop-test-token -v "${RepositoryRoot}:/src" -w /src golang:1.25-bookworm go run ./tools/rjsctl queue apply --url http://management:8223 tests/fixtures/queue-basic-updated.yaml | ConvertFrom-Json
				if ($LASTEXITCODE -ne 0 -or $Updated.status -ne 'ready' -or @($Updated.operations | Where-Object action -eq 'update').Count -ne 2) { throw 'safe update apply failed' }
				$StaleBody = @{apiVersion='rabbit-jetstream.io/v1alpha1'; kind='Queue'; metadata=@{name='basic'}; spec=@{subjects=@('basic.>'); replicas=1; storage='file'}} | ConvertTo-Json -Depth 8
				$StaleResponse = Invoke-WebRequest -Method Put -Uri 'http://127.0.0.1:8223/api/v1/queues/basic' -Headers @{Authorization='Bearer desktop-test-token'; 'If-Match'=$OldETag} -ContentType 'application/json' -Body $StaleBody -TimeoutSec 5 -SkipHttpErrorCheck
				if ($StaleResponse.StatusCode -ne 409) { throw "stale Queue revision returned $($StaleResponse.StatusCode), expected 409" }
				$Stream = Invoke-RestMethod -Uri 'http://127.0.0.1:8223/api/v1/streams/RJSQ_basic' -TimeoutSec 5
				$Consumers = Invoke-RestMethod -Uri 'http://127.0.0.1:8223/api/v1/streams/RJSQ_basic/consumers' -TimeoutSec 5
				if ($Stream.replicas -ne 1 -or $Stream.max_bytes -ne 16777216 -or $Consumers.total -ne 1 -or $Consumers.items[0].name -ne 'RJSQC_basic' -or $Consumers.items[0].max_deliver -ne 7) { throw 'applied resources are incorrect' }
				$Declarations = Invoke-RestMethod -Uri 'http://127.0.0.1:8223/api/v1/queues' -TimeoutSec 5
				if ($Declarations.total -ne 1 -or $Declarations.items[0].queue -ne 'basic' -or $Declarations.items[0].revision -ne $Updated.revision) { throw 'Queue declaration was not persisted' }
				Invoke-Docker compose -p $Project -f $Compose restart management
				Wait-Healthy "${Project}-management-1"
				$Declarations = Invoke-RestMethod -Uri 'http://127.0.0.1:8223/api/v1/queues' -TimeoutSec 10
				if ($Declarations.total -ne 1 -or $Declarations.items[0].queue -ne 'basic') { throw 'Queue declaration did not survive management restart' }
			}
			if ($Scenario -eq 'delete') {
				$Network = "${Project}_default"
				Invoke-Docker run --rm --network $Network -e RJS_ADMIN_TOKEN=desktop-test-token -v "${RepositoryRoot}:/src" -w /src golang:1.25-bookworm go run ./tools/rjsctl queue apply --url http://management:8223 tests/fixtures/queue-basic.yaml
				Invoke-Docker run --rm --network $Network natsio/nats-box:latest nats --server nats://nats:4222 publish basic.test retained-message
				& docker run --rm --network $Network -e RJS_ADMIN_TOKEN=desktop-test-token -v "${RepositoryRoot}:/src" -w /src golang:1.25-bookworm go run ./tools/rjsctl queue delete --url http://management:8223 --confirm basic basic 2>$null
				if ($LASTEXITCODE -eq 0) { throw 'non-empty Queue deletion unexpectedly succeeded without force' }
				$Stream = Invoke-RestMethod -Uri 'http://127.0.0.1:8223/api/v1/streams/RJSQ_basic' -TimeoutSec 5
				if ($Stream.messages -ne 1) { throw 'blocked delete changed the Stream' }
				$Deleted = & docker run --rm --network $Network -e RJS_ADMIN_TOKEN=desktop-test-token -v "${RepositoryRoot}:/src" -w /src golang:1.25-bookworm go run ./tools/rjsctl queue delete --url http://management:8223 --confirm basic --force basic | ConvertFrom-Json
				if ($LASTEXITCODE -ne 0 -or $Deleted.status -ne 'deleted' -or $Deleted.messages -ne 1) { throw 'forced delete failed' }
				$Again = & docker run --rm --network $Network -e RJS_ADMIN_TOKEN=desktop-test-token -v "${RepositoryRoot}:/src" -w /src golang:1.25-bookworm go run ./tools/rjsctl queue delete --url http://management:8223 --confirm basic basic | ConvertFrom-Json
				if ($LASTEXITCODE -ne 0 -or $Again.status -ne 'noop') { throw 'repeated delete was not idempotent' }
				$Declarations = Invoke-RestMethod -Uri 'http://127.0.0.1:8223/api/v1/queues' -TimeoutSec 5
				if ($Declarations.total -ne 0) { throw 'Queue declaration was not deleted' }
				Invoke-Docker run --rm --network $Network natsio/nats-box:latest nats --server nats://nats:4222 stream add RJSQ_foreign --subjects foreign.serve --storage file --replicas 1 --defaults
				& docker run --rm --network $Network -e RJS_ADMIN_TOKEN=desktop-test-token -v "${RepositoryRoot}:/src" -w /src golang:1.25-bookworm go run ./tools/rjsctl queue delete --url http://management:8223 --confirm foreign --force foreign 2>$null
				if ($LASTEXITCODE -eq 0) { throw 'foreign Stream deletion unexpectedly succeeded' }
				$Foreign = Invoke-RestMethod -Uri 'http://127.0.0.1:8223/api/v1/streams/RJSQ_foreign' -TimeoutSec 5
				if ($Foreign.name -ne 'RJSQ_foreign') { throw 'foreign Stream ownership protection failed' }
			}
			if ($Scenario -eq 'routing') {
				$Network = "${Project}_default"
				foreach ($Fixture in @('queue-routing-direct.yaml', 'queue-routing-topic.yaml', 'queue-routing-fanout-a.yaml', 'queue-routing-fanout-b.yaml')) {
					Invoke-Docker run --rm --network $Network -e RJS_ADMIN_TOKEN=desktop-test-token -v "${RepositoryRoot}:/src" -w /src golang:1.25-bookworm go run ./tools/rjsctl queue apply --url http://management:8223 "tests/fixtures/$Fixture"
				}
				Invoke-Docker run --rm --network $Network natsio/nats-box:latest nats --server nats://nats:4222 publish rjs.q.routing_direct.x.commerce.direct.orders.created direct-match
				Invoke-Docker run --rm --network $Network natsio/nats-box:latest nats --server nats://nats:4222 publish rjs.q.routing_direct.x.commerce.direct.orders.deleted direct-miss
				Invoke-Docker run --rm --network $Network natsio/nats-box:latest nats --server nats://nats:4222 publish rjs.q.routing_topic.x.commerce.topic.orders.created topic-star-match
				Invoke-Docker run --rm --network $Network natsio/nats-box:latest nats --server nats://nats:4222 publish rjs.q.routing_topic.x.commerce.topic.orders.created.eu topic-star-miss
				Invoke-Docker run --rm --network $Network natsio/nats-box:latest nats --server nats://nats:4222 publish rjs.q.routing_topic.x.commerce.topic.audit topic-hash-zero
				Invoke-Docker run --rm --network $Network natsio/nats-box:latest nats --server nats://nats:4222 publish rjs.q.routing_topic.x.commerce.topic.audit.security.login topic-hash-many
				# Native SDK/gateway resolves fanout to both queue-scoped targets.
				Invoke-Docker run --rm --network $Network natsio/nats-box:latest nats --server nats://nats:4222 publish rjs.q.routing_fanout_a.x.announcements.fanout fanout-message
				Invoke-Docker run --rm --network $Network natsio/nats-box:latest nats --server nats://nats:4222 publish rjs.q.routing_fanout_b.x.announcements.fanout fanout-message
				$Direct = Invoke-RestMethod -Uri 'http://127.0.0.1:8223/api/v1/streams/RJSQ_routing_direct' -TimeoutSec 5
				$Topic = Invoke-RestMethod -Uri 'http://127.0.0.1:8223/api/v1/streams/RJSQ_routing_topic' -TimeoutSec 5
				$FanoutA = Invoke-RestMethod -Uri 'http://127.0.0.1:8223/api/v1/streams/RJSQ_routing_fanout_a' -TimeoutSec 5
				$FanoutB = Invoke-RestMethod -Uri 'http://127.0.0.1:8223/api/v1/streams/RJSQ_routing_fanout_b' -TimeoutSec 5
				if ($Direct.messages -ne 1) { throw "direct routing stored $($Direct.messages), expected 1" }
				if ($Topic.messages -ne 3) { throw "topic routing stored $($Topic.messages), expected 3" }
				if ($FanoutA.messages -ne 1 -or $FanoutB.messages -ne 1) { throw 'fanout did not copy to both Queues' }
			}
			if ($Scenario -eq 'dlq') {
				$Network = "${Project}_default"
				foreach ($Fixture in @('queue-dlq-target.yaml', 'queue-dlq-source.yaml')) {
					Invoke-Docker run --rm --network $Network -e RJS_ADMIN_TOKEN=desktop-test-token -v "${RepositoryRoot}:/src" -w /src golang:1.25-bookworm go run ./tools/rjsctl queue apply --url http://management:8223 "tests/fixtures/$Fixture"
				}
				Invoke-Docker run --rm --network $Network natsio/nats-box:latest nats --server nats://nats:4222 publish rjs.q.retry_orders.ingress poison-message
				Invoke-Docker run --rm --network $Network natsio/nats-box:latest nats --server nats://nats:4222 consumer next RJSQ_retry_orders RJSQC_retry_orders --no-ack --wait 1s
				Start-Sleep -Seconds 2
				Invoke-Docker run --rm --network $Network natsio/nats-box:latest nats --server nats://nats:4222 consumer next RJSQ_retry_orders RJSQC_retry_orders --no-ack --wait 1s
				Start-Sleep -Seconds 2
				$Moved = $false
				for ($Attempt = 0; $Attempt -lt 15; $Attempt++) {
					$Source = Invoke-RestMethod -Uri 'http://127.0.0.1:8223/api/v1/streams/RJSQ_retry_orders' -TimeoutSec 5
					$Target = Invoke-RestMethod -Uri 'http://127.0.0.1:8223/api/v1/streams/RJSQ_failed_orders' -TimeoutSec 5
					$Controller = Invoke-RestMethod -Uri 'http://127.0.0.1:8223/api/v1/controller' -TimeoutSec 5
					if ($Source.messages -eq 0 -and $Target.messages -eq 1 -and $Controller.dlqMoved -ge 1) { $Moved = $true; break }
					Start-Sleep -Seconds 1
				}
				if (-not $Moved) { throw "DLQ transfer failed: source=$($Source.messages) target=$($Target.messages) moved=$($Controller.dlqMoved) error=$($Controller.lastError)" }
			}
            Invoke-Docker compose -p $Project -f $Compose ps
        } finally {
            Invoke-Docker compose -p $Project -f $Compose down -v --remove-orphans
			$env:RJS_ADMIN_TOKEN = $PreviousAdminToken
        }
        return
    }

    if ($Scenario -eq 'controller') {
        $Project = 'rjs-desktop-controller'
        $Compose = 'deploy/compose/cluster.yml'
        $Network = "${Project}_default"
        $Second = "${Project}-management-2"
        $PreviousAdminToken = $env:RJS_ADMIN_TOKEN
        $PreviousInterval = $env:RJS_CONTROLLER_INTERVAL
        $PreviousLeaseTTL = $env:RJS_CONTROLLER_LEASE_TTL
        $env:RJS_ADMIN_TOKEN = 'desktop-test-token'
        $env:RJS_CONTROLLER_INTERVAL = '1s'
        $env:RJS_CONTROLLER_LEASE_TTL = '4s'
        try {
            Invoke-Docker compose -p $Project -f $Compose up -d --build --wait
            Invoke-Docker run -d --name $Second --network $Network -p 28223:8223 -e 'RJS_NATS_URL=nats://nats-1:4222,nats://nats-2:4222,nats://nats-3:4222' -e 'RJS_NATS_MONITOR_URLS=http://nats-1:8222,http://nats-2:8222,http://nats-3:8222' -e RJS_ADMIN_TOKEN=desktop-test-token -e RJS_METADATA_REPLICAS=3 -e RJS_INSTANCE_ID=management-2 -e RJS_CONTROLLER_INTERVAL=1s -e RJS_CONTROLLER_LEASE_TTL=4s rabbit-jetstream/management:local
            $Apply = & docker run --rm --network $Network -e RJS_ADMIN_TOKEN=desktop-test-token -v "${RepositoryRoot}:/src" -w /src golang:1.25-bookworm go run ./tools/rjsctl queue apply --url http://management:8223 tests/fixtures/queue-cluster.yaml | ConvertFrom-Json
            if ($LASTEXITCODE -ne 0 -or $Apply.status -ne 'ready') { throw 'cluster Queue apply failed' }
            $OldETag = [string]((Invoke-WebRequest -Uri 'http://127.0.0.1:8223/api/v1/queues/cluster' -TimeoutSec 5).Headers.ETag | Select-Object -First 1)
            $Updated = & docker run --rm --network $Network -e RJS_ADMIN_TOKEN=desktop-test-token -v "${RepositoryRoot}:/src" -w /src golang:1.25-bookworm go run ./tools/rjsctl queue apply --url http://management:8223 tests/fixtures/queue-cluster-updated.yaml | ConvertFrom-Json
            if ($LASTEXITCODE -ne 0 -or $Updated.status -ne 'ready') { throw 'cluster Queue update failed' }
            $StaleBody = @{apiVersion='rabbit-jetstream.io/v1alpha1'; kind='Queue'; metadata=@{name='cluster'}; spec=@{subjects=@('cluster.>'); replicas=3; storage='file'}} | ConvertTo-Json -Depth 8
            $StaleResponse = Invoke-WebRequest -Method Put -Uri 'http://127.0.0.1:28223/api/v1/queues/cluster' -Headers @{Authorization='Bearer desktop-test-token'; 'If-Match'=$OldETag} -ContentType 'application/json' -Body $StaleBody -TimeoutSec 5 -SkipHttpErrorCheck
            if ($StaleResponse.StatusCode -ne 409) { throw "cross-instance stale update returned $($StaleResponse.StatusCode), expected 409" }
            $SharedDeclaration = Invoke-RestMethod -Uri 'http://127.0.0.1:28223/api/v1/queues/cluster' -TimeoutSec 5
            if ($SharedDeclaration.revision -ne $Updated.revision) { throw 'management instances do not share the latest declaration revision' }
            Start-Sleep -Seconds 3
            $FirstStatus = Get-NetworkJson $Network 'http://management:8223/api/v1/controller'
            $SecondStatus = Get-NetworkJson $Network "http://${Second}:8223/api/v1/controller"
            if ([int]$FirstStatus.leader + [int]$SecondStatus.leader -ne 1) { throw 'expected exactly one controller leader' }
            if ($FirstStatus.leader) {
                Invoke-Docker compose -p $Project -f $Compose stop management
                $SurvivorController = "http://${Second}:8223/api/v1/controller"
            } else {
                Invoke-Docker stop $Second
                $SurvivorController = 'http://management:8223/api/v1/controller'
            }
            $Leader = Wait-NetworkControllerLeader $Network $SurvivorController
            Invoke-Docker run --rm --network $Network natsio/nats-box:latest nats --server nats://nats-1:4222 consumer rm RJSQ_cluster RJSQC_cluster --force
            $Recovered = $false
            foreach ($Attempt in 1..20) {
                $Consumer = & docker run --rm --network $Network natsio/nats-box:latest nats --server nats://nats-1:4222 consumer info RJSQ_cluster RJSQC_cluster --json 2>$null | ConvertFrom-Json
                if ($LASTEXITCODE -eq 0 -and $Consumer.name -eq 'RJSQC_cluster') { $Recovered = $true; break }
                Start-Sleep -Seconds 1
            }
            if (-not $Recovered) { throw 'controller did not recreate the deleted Consumer' }
        } finally {
            & docker rm -f $Second 2>$null | Out-Null
            Invoke-Docker compose -p $Project -f $Compose down -v --remove-orphans
            $env:RJS_ADMIN_TOKEN = $PreviousAdminToken
            $env:RJS_CONTROLLER_INTERVAL = $PreviousInterval
            $env:RJS_CONTROLLER_LEASE_TTL = $PreviousLeaseTTL
        }
        return
    }

    $Project = 'rjs-desktop-fault'
    $Compose = 'deploy/compose/cluster.yml'
    $Network = "${Project}_default"
    try {
        Invoke-Docker compose -p $Project -f $Compose up -d --build --wait
        $Nodes = Invoke-RestMethod -Uri 'http://127.0.0.1:8223/api/v1/nodes' -TimeoutSec 5
        if ($Nodes.status -ne 'available' -or $Nodes.available -ne 3) { throw 'initial node monitoring is incorrect' }
        Invoke-Docker run --rm --network $Network natsio/nats-box:latest nats --server nats://nats-1:4222 stream add RJS_E2E --subjects rjs.e2e --storage file --replicas 3 --defaults
        Invoke-Docker run --rm --network $Network natsio/nats-box:latest nats --server nats://nats-1:4222 publish rjs.e2e before-failure
        Invoke-Docker compose -p $Project -f $Compose stop nats-1
        Start-Sleep -Seconds 5
        Invoke-Docker run --rm --network $Network natsio/nats-box:latest nats --server nats://nats-2:4222 publish rjs.e2e during-failure
        $Ready = Invoke-RestMethod -Uri 'http://127.0.0.1:8223/readyz' -TimeoutSec 5
        if ($Ready.status -ne 'ready') { throw 'management API failed during node outage' }
        $Nodes = Invoke-RestMethod -Uri 'http://127.0.0.1:8223/api/v1/nodes' -TimeoutSec 8
        if ($Nodes.status -ne 'degraded' -or $Nodes.available -ne 2 -or $Nodes.unavailable -ne 1) { throw 'node outage was not reported correctly' }
        Invoke-Docker compose -p $Project -f $Compose start nats-1
        Wait-Healthy "${Project}-nats-1-1"
        $Info = & docker run --rm --network $Network natsio/nats-box:latest nats --server nats://nats-1:4222 stream info RJS_E2E --json | ConvertFrom-Json
        if ($LASTEXITCODE -ne 0) { throw 'stream inspection failed' }
        if ($Info.state.messages -ne 2) { throw "expected 2 messages, got $($Info.state.messages)" }
        $CurrentReplicas = @($Info.cluster.replicas | Where-Object current).Count
        if ($CurrentReplicas -ne 2) { throw "expected 2 current followers, got $CurrentReplicas" }
        $ManagedStream = Invoke-RestMethod -Uri 'http://127.0.0.1:8223/api/v1/streams/RJS_E2E' -TimeoutSec 5
        if ($ManagedStream.messages -ne 2) { throw 'management API message count is incorrect' }
        if ([string]::IsNullOrWhiteSpace($ManagedStream.cluster.leader)) { throw 'management API did not report a leader' }
        if (@($ManagedStream.cluster.replicas | Where-Object current).Count -ne 2) { throw 'management API replica state is incorrect' }
        $Nodes = Invoke-RestMethod -Uri 'http://127.0.0.1:8223/api/v1/nodes' -TimeoutSec 8
        if ($Nodes.status -ne 'available' -or $Nodes.available -ne 3) { throw 'recovered node monitoring is incorrect' }
    } finally {
        Invoke-Docker compose -p $Project -f $Compose down -v --remove-orphans
    }
} finally {
    Pop-Location
}
