#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cluster="rjs-smoke-${GITHUB_RUN_ID:-local}-${GITHUB_RUN_ATTEMPT:-1}"
cluster="${cluster,,}"
cluster="${cluster//_/-}"
kind_bin="$(go env GOPATH)/bin/kind"
helm_image='alpine/helm:3.18.4@sha256:e7ecbf4a200dea73d64bfb8cb0936829164945f2b4d02a0274093073ee8d264f'
node_image='kindest/node:v1.35.0@sha256:452d707d4862f52530247495d180205e029056831160e22870e37e3f6c1ac31f'
namespace='rjs-kubernetes-smoke'
release='rjs-smoke'
local_port="${RJS_KUBERNETES_SMOKE_PORT:-18223}"
temporary="$(mktemp -d)"
port_forward_pid=''

cleanup() {
  if [[ -n "$port_forward_pid" ]]; then kill "$port_forward_pid" 2>/dev/null || true; fi
  "$kind_bin" delete cluster --name "$cluster" >/dev/null 2>&1 || true
  rm -rf -- "$temporary"
}
trap cleanup EXIT

command -v docker >/dev/null
command -v go >/dev/null
command -v kubectl >/dev/null
if [[ ! -x "$kind_bin" ]]; then
  go install sigs.k8s.io/kind@v0.31.0
fi

"$kind_bin" delete cluster --name "$cluster" >/dev/null 2>&1 || true
"$kind_bin" create cluster --name "$cluster" --image "$node_image" --config "$repo_root/tests/deployment/kind.yaml" --wait 5m
docker build -f "$repo_root/packaging/Dockerfile.nats-server" -t rabbit-jetstream/nats-server:kubernetes-smoke "$repo_root"
docker build -f "$repo_root/packaging/Dockerfile.management" -t rabbit-jetstream/management:kubernetes-smoke "$repo_root"
"$kind_bin" load docker-image --name "$cluster" rabbit-jetstream/nats-server:kubernetes-smoke rabbit-jetstream/management:kubernetes-smoke

docker run --rm --network host \
  -v "$HOME/.kube:/root/.kube:ro" -v "$repo_root:/src:ro" \
  "$helm_image" upgrade --install "$release" /src/deploy/helm/rabbit-jetstream \
  --namespace "$namespace" --create-namespace \
  --set nats.image.tag=kubernetes-smoke --set nats.image.pullPolicy=Never \
  --set management.image.tag=kubernetes-smoke --set management.image.pullPolicy=Never \
  --set nats.storage.storageClass=standard --wait --timeout 8m

kubectl -n "$namespace" rollout status "statefulset/${release}-rabbit-jetstream-nats" --timeout=5m
kubectl -n "$namespace" rollout status "deployment/${release}-rabbit-jetstream-management" --timeout=5m
test "$(kubectl -n "$namespace" get pod -l app.kubernetes.io/component=nats -o jsonpath='{range .items[*]}{.spec.nodeName}{"\n"}{end}' | sort -u | grep -c .)" -eq 3
test "$(kubectl -n "$namespace" get pvc -l app.kubernetes.io/instance="$release" -o jsonpath='{range .items[*]}{.status.phase}{"\n"}{end}' | grep -c '^Bound$')" -eq 3

docker run --rm --network host -v "$HOME/.kube:/root/.kube:ro" \
  "$helm_image" test "$release" --namespace "$namespace" --logs --timeout 3m

kubectl -n "$namespace" port-forward --address 127.0.0.1 "service/${release}-rabbit-jetstream-management" "$local_port:8223" >"$temporary/port-forward.log" 2>&1 &
port_forward_pid=$!
for _ in $(seq 1 30); do
  if curl --fail --silent --show-error "http://127.0.0.1:$local_port/readyz" | grep -q '"status":"ready"'; then
    curl --fail --silent --show-error "http://127.0.0.1:$local_port/admin/" | grep -q '<title>Rabbit JetStream · Operations</title>'
    exit 0
  fi
  sleep 1
done
cat "$temporary/port-forward.log" >&2
kubectl -n "$namespace" get pods -o wide >&2
exit 1
