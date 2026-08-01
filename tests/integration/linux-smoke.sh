#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
compose_file="$repo_root/deploy/compose/standalone.yml"
project="rjs-linux-smoke-${GITHUB_RUN_ID:-local}"
bundle="$repo_root/.tmp-rjs-diagnostics-${GITHUB_RUN_ID:-local}.zip"

cleanup() {
  rm -f "$bundle"
  docker compose -p "$project" -f "$compose_file" down -v --remove-orphans
}
trap cleanup EXIT

docker compose -p "$project" -f "$compose_file" up -d --build --wait
curl --fail --silent --show-error http://127.0.0.1:8223/healthz
curl --fail --silent --show-error http://127.0.0.1:8223/readyz
curl --fail --silent --show-error http://127.0.0.1:8223/api/v1/info

network="${project}_default"
docker run --rm --network "$network" natsio/nats-box:latest \
  nats --server nats://nats:4222 stream add RJS_API --subjects rjs.api --storage file --replicas 1 --defaults
docker run --rm --network "$network" natsio/nats-box:latest \
  nats --server nats://nats:4222 consumer add RJS_API WORKER --filter rjs.api --ack explicit --pull --defaults
curl --fail --silent --show-error http://127.0.0.1:8223/api/v1/cluster | grep -q '"streams":1'
curl --fail --silent --show-error http://127.0.0.1:8223/api/v1/streams | grep -q '"name":"RJS_API"'
curl --fail --silent --show-error http://127.0.0.1:8223/api/v1/streams/RJS_API | grep -q '"replicas":1'
curl --fail --silent --show-error http://127.0.0.1:8223/api/v1/streams/RJS_API/consumers | grep -q '"name":"WORKER"'
curl --fail --silent --show-error http://127.0.0.1:8223/api/v1/nodes | grep -q '"available":1'

docker run --rm --network "$network" -v "$repo_root:/src" -w /src golang:1.25-bookworm \
  go run ./tools/rjsctl diagnostics collect --url http://management:8223 \
  --output "/src/$(basename "$bundle")"
unzip -t "$bundle"
unzip -p "$bundle" manifest.json | grep -q 'rabbit-jetstream.io/diagnostics/v1alpha1'
unzip -p "$bundle" manifest.json | grep -q '"sha256"'

docker image inspect rabbit-jetstream/nats-server:local \
  --format '{{if ne .Os "linux"}}{{json .}}{{end}}' | grep -q '^$'
docker image inspect rabbit-jetstream/management:local \
  --format '{{if ne .Os "linux"}}{{json .}}{{end}}' | grep -q '^$'
