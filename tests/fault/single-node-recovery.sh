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

wait_for_nodes() {
  local expected="$1"
  local response=''
  for _ in $(seq 1 45); do
    response="$(curl --fail --silent --show-error -H 'Authorization: Bearer fault-test-token' http://127.0.0.1:8223/api/v1/nodes || true)"
    if grep -F "$expected" >/dev/null <<<"$response"; then return 0; fi
    sleep 1
  done
  printf 'node state did not reach %s; last response: %s\n' "$expected" "$response" >&2
  return 1
}

export RJS_ADMIN_TOKEN=fault-test-token
docker compose -p "$project" -f "$compose_file" up -d --build --wait
wait_for_nodes '"available":3'
nats nats-1 stream add RJS_E2E --subjects rjs.e2e --storage file --replicas 3 --defaults
nats nats-1 publish rjs.e2e before-failure
for _ in $(seq 1 45); do
  baseline_json="$(nats nats-1 stream info RJS_E2E --json || true)"
  if jq -e '(.state.messages == 1) and (([.cluster.replicas[]? | select(.current == true)] | length) == 2)' >/dev/null <<<"$baseline_json"; then
    break
  fi
  sleep 1
done
jq -e '(.state.messages == 1) and (([.cluster.replicas[]? | select(.current == true)] | length) == 2)' >/dev/null <<<"$baseline_json"

docker compose -p "$project" -f "$compose_file" stop nats-1
sleep 5
nats nats-2 publish rjs.e2e during-failure
curl --fail --silent --show-error http://127.0.0.1:8223/readyz
nodes_during_failure="$(curl --fail --silent --show-error -H 'Authorization: Bearer fault-test-token' http://127.0.0.1:8223/api/v1/nodes)"
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
for _ in $(seq 1 45); do
  cluster_json="$(nats nats-1 stream info RJS_E2E --json || true)"
  managed_stream="$(curl --fail --silent --show-error -H 'Authorization: Bearer fault-test-token' http://127.0.0.1:8223/api/v1/streams/RJS_E2E || true)"
  if jq -e '(.state.messages == 2) and (([.cluster.replicas[]? | select(.current == true)] | length) == 2)' >/dev/null <<<"$cluster_json" &&
     jq -e '(.messages == 2) and (([.cluster.replicas[]? | select(.current == true)] | length) == 2)' >/dev/null <<<"$managed_stream"; then
    break
  fi
  sleep 1
done
jq -e '(.state.messages == 2) and (([.cluster.replicas[]? | select(.current == true)] | length) == 2)' >/dev/null <<<"$cluster_json"
jq -e '(.messages == 2) and (([.cluster.replicas[]? | select(.current == true)] | length) == 2)' >/dev/null <<<"$managed_stream"
wait_for_nodes '"available":3'
