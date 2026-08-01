#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
compose_file="$repo_root/deploy/compose/standalone.yml"
project="rjs-linux-smoke-${GITHUB_RUN_ID:-local}"

cleanup() {
  docker compose -p "$project" -f "$compose_file" down -v --remove-orphans
}
trap cleanup EXIT

docker compose -p "$project" -f "$compose_file" up -d --build --wait
curl --fail --silent --show-error http://127.0.0.1:8223/healthz
curl --fail --silent --show-error http://127.0.0.1:8223/readyz
curl --fail --silent --show-error http://127.0.0.1:8223/api/v1/info

docker image inspect rabbit-jetstream/nats-server:local \
  --format '{{if ne .Os "linux"}}{{json .}}{{end}}' | grep -q '^$'
docker image inspect rabbit-jetstream/management:local \
  --format '{{if ne .Os "linux"}}{{json .}}{{end}}' | grep -q '^$'
