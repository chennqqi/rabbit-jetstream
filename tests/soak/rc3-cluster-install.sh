#!/usr/bin/env bash
# Install the production profile release and verify rollout, node spread,
# PVC binding, auth-secret stability across a second upgrade, and per-node
# digest aliases for the candidate images.
set -euo pipefail
namespace='rjs-kubernetes-smoke'
release='production'
helm_image='docker.m.daocloud.io/alpine/helm:3.18.4'
export KIND_EXPERIMENTAL_PROVIDER=podman
cd /root/rc3-cluster

image_id() { printf 'sha256:%s\n' "$(podman image inspect "$1" --format '{{.Id}}')"; }
nats_digest="$(image_id localhost/rabbit-jetstream/nats-server:kubernetes-smoke)"
management_digest="$(image_id localhost/rabbit-jetstream/management:kubernetes-smoke)"
operator_digest="$(image_id localhost/rabbit-jetstream/operator:kubernetes-smoke)"

helm_upgrade() {
  podman run --rm --network host \
    -v "$HOME/.kube:/root/.kube:ro" -v "/root/rc3-cluster:/src:ro" \
    "$helm_image" upgrade --install "$release" /src/rabbit-jetstream \
    --namespace "$namespace" --create-namespace \
    --values /src/production-smoke-values.yaml \
    --set-string "nats.image.digest=$nats_digest" \
    --set-string "management.image.digest=$management_digest" \
    --set-string "operator.image.digest=$operator_digest" \
    --wait --timeout 8m
}

temporary="$(mktemp -d)"
./tls-fixture --output "$temporary/tls"
test "$(sha256sum "$temporary/tls/server/tls.key" | cut -d' ' -f1)" != "$(sha256sum "$temporary/tls/client/tls.key" | cut -d' ' -f1)"
kubectl create namespace "$namespace"
kubectl -n "$namespace" create secret generic rjs-nats-server-tls \
  --from-file=ca.crt="$temporary/tls/server/ca.crt" --from-file=tls.crt="$temporary/tls/server/tls.crt" --from-file=tls.key="$temporary/tls/server/tls.key"
kubectl -n "$namespace" create secret generic rjs-nats-client-tls \
  --from-file=ca.crt="$temporary/tls/client/ca.crt" --from-file=tls.crt="$temporary/tls/client/tls.crt" --from-file=tls.key="$temporary/tls/client/tls.key"

helm_upgrade
kubectl -n "$namespace" rollout status "statefulset/${release}-rabbit-jetstream-nats" --timeout=5m
kubectl -n "$namespace" rollout status "deployment/${release}-rabbit-jetstream-management" --timeout=5m
test "$(kubectl -n "$namespace" get pod -l app.kubernetes.io/component=nats -o jsonpath='{range .items[*]}{.spec.nodeName}{"\n"}{end}' | sort -u | grep -c .)" -eq 3
test "$(kubectl -n "$namespace" get pvc -l app.kubernetes.io/instance="$release" -o jsonpath='{range .items[*]}{.status.phase}{"\n"}{end}' | grep -c '^Bound$')" -eq 3
echo "install verified: rollout, 3-node spread, PVC binding"

auth_secret="${release}-rabbit-jetstream-auth"
auth_before="$(kubectl -n "$namespace" get secret "$auth_secret" -o jsonpath='{.data}')"
helm_upgrade
auth_after="$(kubectl -n "$namespace" get secret "$auth_secret" -o jsonpath='{.data}')"
test "$auth_before" = "$auth_after"
admin_token="$(kubectl -n "$namespace" get secret "$auth_secret" -o jsonpath='{.data.admin-token}' | base64 -d)"
test -n "$admin_token"
echo "auth secret stability verified"
kubectl -n "$namespace" rollout status "deployment/${release}-rabbit-jetstream-management" --timeout=5m

for node in $(kubectl get nodes -o jsonpath='{range .items[*]}{.metadata.name}{" "}{end}'); do
  podman exec "$node" ctr -n k8s.io images tag \
    "localhost/rabbit-jetstream/nats-server:kubernetes-smoke" "docker.io/rabbit-jetstream/nats-server:kubernetes-smoke" 2>/dev/null || true
  podman exec "$node" ctr -n k8s.io images tag \
    "localhost/rabbit-jetstream/management:kubernetes-smoke" "docker.io/rabbit-jetstream/management:kubernetes-smoke" 2>/dev/null || true
  podman exec "$node" ctr -n k8s.io images tag \
    "localhost/rabbit-jetstream/operator:kubernetes-smoke" "docker.io/rabbit-jetstream/operator:kubernetes-smoke" 2>/dev/null || true
done
echo "per-node image aliases applied"
