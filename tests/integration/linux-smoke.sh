#!/usr/bin/env bash
set -euo pipefail

go_tool_image='golang@sha256:ea341baa9bd5ba6784f6d7161ace70544349a6242d54d34a0fbfd2c4d51c9d58'
nats_box_image='natsio/nats-box@sha256:ffce8bd103383f179f8c7f11cf645726acf5d17280706c530c3b342dbe16334c'
alpine_image='alpine:3.23@sha256:fd791d74b68913cbb027c6546007b3f0d3bc45125f797758156952bc2d6daf40'

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
compose_file="$repo_root/deploy/compose/standalone.yml"
project="rjs-linux-smoke-${GITHUB_RUN_ID:-local}"
bundle="$repo_root/.tmp-rjs-diagnostics-${GITHUB_RUN_ID:-local}.zip"

cleanup() {
  rm -f "$bundle"
  docker compose -p "$project" -f "$compose_file" down -v --remove-orphans
}
trap cleanup EXIT

wait_for_api() {
  local path="$1"
  local expected="$2"
  local response=''
  for _ in $(seq 1 30); do
    response="$(curl --fail --silent --show-error "http://127.0.0.1:8223${path}" || true)"
    if grep -F "$expected" >/dev/null <<<"$response"; then
      return 0
    fi
    sleep 1
  done
  printf 'management API did not expose %s at %s; last response: %s\n' "$expected" "$path" "$response" >&2
  return 1
}

docker compose -p "$project" -f "$compose_file" up -d --build --wait
curl --fail --silent --show-error http://127.0.0.1:8223/healthz
curl --fail --silent --show-error http://127.0.0.1:8223/readyz
curl --fail --silent --show-error http://127.0.0.1:8223/api/v1/info

network="${project}_default"
docker run --rm --network "$network" "$nats_box_image" \
  nats --server nats://nats:4222 stream add RJS_API --subjects rjs.api --storage file --replicas 1 --defaults
docker run --rm --network "$network" "$nats_box_image" \
  nats --server nats://nats:4222 consumer add RJS_API WORKER --filter rjs.api --ack explicit --pull --defaults
wait_for_api /api/v1/cluster '"streams":2'
wait_for_api /api/v1/streams '"name":"RJS_API"'
wait_for_api /api/v1/streams/RJS_API '"replicas":1'
wait_for_api /api/v1/streams/RJS_API/consumers '"name":"WORKER"'
wait_for_api /api/v1/nodes '"available":1'

docker run --rm --network "$network" -v "$repo_root:/src" -w /src "$go_tool_image" \
  go run ./tools/rjsctl diagnostics collect --url http://management:8223 \
  --output "/src/$(basename "$bundle")"
docker run --rm -v "$bundle:/bundle.zip" "$alpine_image" chmod a+r /bundle.zip
unzip -t "$bundle"
unzip -p "$bundle" manifest.json | grep -F 'rabbit-jetstream.io/diagnostics/v1alpha1' >/dev/null
unzip -p "$bundle" manifest.json | grep -F '"sha256"' >/dev/null

test "$(docker image inspect rabbit-jetstream/nats-server:local --format '{{.Os}}')" = linux
test "$(docker image inspect rabbit-jetstream/management:local --format '{{.Os}}')" = linux
