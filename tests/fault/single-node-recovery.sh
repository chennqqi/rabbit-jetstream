#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
compose_file="$repo_root/deploy/compose/cluster.yml"
project="rjs-linux-fault-${GITHUB_RUN_ID:-local}"
network="${project}_default"
nats_box="natsio/nats-box@sha256:ffce8bd103383f179f8c7f11cf645726acf5d17280706c530c3b342dbe16334c"

cleanup() {
  docker compose -p "$project" -f "$compose_file" down -v --remove-orphans
}
trap cleanup EXIT

nats() {
  local server="$1"
  shift
  docker run --rm --network "$network" "$nats_box" \
    nats --server "nats://$server:4222" "$@"
}

docker compose -p "$project" -f "$compose_file" up -d --build --wait
curl --fail --silent --show-error http://127.0.0.1:8223/api/v1/nodes | grep -F '"available":3' >/dev/null
nats nats-1 stream add RJS_E2E --subjects rjs.e2e --storage file --replicas 3 --defaults
nats nats-1 publish rjs.e2e before-failure

docker compose -p "$project" -f "$compose_file" stop nats-1
sleep 5
nats nats-2 publish rjs.e2e during-failure
curl --fail --silent --show-error http://127.0.0.1:8223/readyz
nodes_during_failure="$(curl --fail --silent --show-error http://127.0.0.1:8223/api/v1/nodes)"
test "$(printf '%s' "$nodes_during_failure" | grep -c '"status":"degraded"')" -eq 1
test "$(printf '%s' "$nodes_during_failure" | grep -c '"unavailable":1')" -eq 1

messages="$(nats nats-2 stream info RJS_E2E --json | tr -d '\r\n' | sed -n 's/.*"messages": *\([0-9][0-9]*\).*/\1/p')"
test "$messages" -eq 2

docker compose -p "$project" -f "$compose_file" start nats-1
for _ in $(seq 1 45); do
  if [[ "$(docker inspect --format '{{.State.Health.Status}}' "${project}-nats-1-1")" == "healthy" ]]; then
    break
  fi
  sleep 1
done
test "$(docker inspect --format '{{.State.Health.Status}}' "${project}-nats-1-1")" = "healthy"
sleep 3

cluster_json="$(nats nats-1 stream info RJS_E2E --json)"
test "$(printf '%s' "$cluster_json" | grep -c '"current": true')" -eq 2
test "$(printf '%s' "$cluster_json" | grep -c '"messages": 2')" -eq 1
managed_stream="$(curl --fail --silent --show-error http://127.0.0.1:8223/api/v1/streams/RJS_E2E)"
test "$(printf '%s' "$managed_stream" | grep -c '"messages":2')" -eq 1
test "$(printf '%s' "$managed_stream" | grep -c '"current":true')" -eq 2
curl --fail --silent --show-error http://127.0.0.1:8223/api/v1/nodes | grep -F '"available":3' >/dev/null
