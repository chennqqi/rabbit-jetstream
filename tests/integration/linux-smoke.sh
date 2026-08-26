#!/usr/bin/env bash
set -euo pipefail

go_tool_image='golang@sha256:ea341baa9bd5ba6784f6d7161ace70544349a6242d54d34a0fbfd2c4d51c9d58'
nats_box_image='natsio/nats-box@sha256:ffce8bd103383f179f8c7f11cf645726acf5d17280706c530c3b342dbe16334c'

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
docker run --rm --network "$network" "$nats_box_image" \
  nats --server nats://nats:4222 stream add RJS_API --subjects rjs.api --storage file --replicas 1 --defaults
docker run --rm --network "$network" "$nats_box_image" \
  nats --server nats://nats:4222 consumer add RJS_API WORKER --filter rjs.api --ack explicit --pull --defaults
curl --fail --silent --show-error http://127.0.0.1:8223/api/v1/cluster | grep -F '"streams":1' >/dev/null
curl --fail --silent --show-error http://127.0.0.1:8223/api/v1/streams | grep -F '"name":"RJS_API"' >/dev/null
curl --fail --silent --show-error http://127.0.0.1:8223/api/v1/streams/RJS_API | grep -F '"replicas":1' >/dev/null
curl --fail --silent --show-error http://127.0.0.1:8223/api/v1/streams/RJS_API/consumers | grep -F '"name":"WORKER"' >/dev/null
curl --fail --silent --show-error http://127.0.0.1:8223/api/v1/nodes | grep -F '"available":1' >/dev/null

docker run --rm --network "$network" -v "$repo_root:/src" -w /src "$go_tool_image" \
  go run ./tools/rjsctl diagnostics collect --url http://management:8223 \
  --output "/src/$(basename "$bundle")"
unzip -t "$bundle"
unzip -p "$bundle" manifest.json | grep -F 'rabbit-jetstream.io/diagnostics/v1alpha1' >/dev/null
unzip -p "$bundle" manifest.json | grep -F '"sha256"' >/dev/null

docker image inspect rabbit-jetstream/nats-server:local \
  --format '{{if ne .Os "linux"}}{{json .}}{{end}}' | grep -q '^$'
docker image inspect rabbit-jetstream/management:local \
  --format '{{if ne .Os "linux"}}{{json .}}{{end}}' | grep -q '^$'
