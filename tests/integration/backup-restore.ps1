$ErrorActionPreference = 'Stop'
$RepositoryRoot = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
$Project = "rjs-desktop-backup-$PID"
$Compose = 'deploy/compose/standalone.yml'
$Network = "${Project}_default"
$BackupRoot = Join-Path $RepositoryRoot ".tmp-rjs-backup-$PID"
$PreviousAdminToken = $env:RJS_ADMIN_TOKEN
$env:RJS_ADMIN_TOKEN = 'desktop-test-token'

function Invoke-Docker {
    & docker @args
    if ($LASTEXITCODE -ne 0) { throw "docker command failed: docker $args" }
}

Push-Location $RepositoryRoot
try {
    Invoke-Docker build -f packaging/Dockerfile.operator -t rabbit-jetstream/operator:local .
    Invoke-Docker compose -p $Project -f $Compose up -d --build --wait
    Invoke-Docker run --rm --network $Network -e RJS_ADMIN_TOKEN=desktop-test-token -v "${RepositoryRoot}:/src" -w /src rabbit-jetstream/operator:local queue apply --url http://management:8223 tests/fixtures/queue-basic.yaml
    foreach ($Message in @('one', 'two', 'three')) {
        Invoke-Docker run --rm --network $Network --entrypoint nats rabbit-jetstream/operator:local --server nats://nats:4222 publish basic.test $Message
    }

    New-Item -ItemType Directory -Path $BackupRoot | Out-Null
    Invoke-Docker run --rm --network $Network -v "${BackupRoot}:/backup" rabbit-jetstream/operator:local backup create --server nats://nats:4222 --output /backup/account
    Invoke-Docker run --rm -v "${BackupRoot}:/backup:ro" rabbit-jetstream/operator:local backup verify --input /backup/account

    Invoke-Docker compose -p $Project -f $Compose down -v --remove-orphans
    Invoke-Docker compose -p $Project -f $Compose up -d nats --wait
    Invoke-Docker run --rm --network $Network -v "${BackupRoot}:/backup:ro" rabbit-jetstream/operator:local backup restore --server nats://nats:4222 --input /backup/account --confirm RESTORE --replicas 1
    Invoke-Docker compose -p $Project -f $Compose up -d management --wait

    $Declarations = Invoke-RestMethod -Uri 'http://127.0.0.1:8223/api/v1/queues' -TimeoutSec 10
    $Stream = Invoke-RestMethod -Uri 'http://127.0.0.1:8223/api/v1/streams/RJSQ_basic' -TimeoutSec 10
    $Consumers = Invoke-RestMethod -Uri 'http://127.0.0.1:8223/api/v1/streams/RJSQ_basic/consumers' -TimeoutSec 10
    if ($Declarations.total -ne 1 -or $Declarations.items[0].queue -ne 'basic') { throw 'Queue declaration was not restored' }
    if ($Stream.messages -ne 3) { throw "restored Stream has $($Stream.messages) messages, expected 3" }
    if ($Consumers.total -ne 1 -or $Consumers.items[0].name -ne 'RJSQC_basic') { throw 'durable Consumer was not restored' }
} finally {
    Invoke-Docker compose -p $Project -f $Compose down -v --remove-orphans
    Remove-Item -LiteralPath $BackupRoot -Recurse -Force -ErrorAction SilentlyContinue
    $env:RJS_ADMIN_TOKEN = $PreviousAdminToken
    Pop-Location
}
