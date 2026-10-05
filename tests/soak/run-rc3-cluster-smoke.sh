#!/usr/bin/env bash
# rc.3 cluster qualification on the native linux/amd64 host (rootless podman).
# Podman adaptation of tests/deployment/kubernetes-smoke.sh: images are
# pre-built from frozen binaries (no source tree on the host), loaded via
# image-archive, and helm runs containerized against the kind cluster.
set -euo pipefail

cluster="rjs-smoke-local-1"
namespace='rjs-kubernetes-smoke'
release='production'
local_port="${RJS_KUBERNETES_SMOKE_PORT:-18223}"
temporary="$(mktemp -d)"
port_forward_pid=''
helm_image='docker.m.daocloud.io/alpine/helm:3.18.4'
busybox_image='docker.m.daocloud.io/library/busybox:1.37.0'
export KIND_EXPERIMENTAL_PROVIDER=podman

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

evict_pod() {
  local pod="$1"
  printf '{"apiVersion":"policy/v1","kind":"Eviction","metadata":{"name":"%s","namespace":"%s"}}\n' "$pod" "$namespace" | \
    kubectl create --raw "/api/v1/namespaces/$namespace/pods/$pod/eviction" -f -
}

wait_for_zero_disruptions() {
  local pdb="$1"
  for _ in $(seq 1 60); do
    if [[ "$(kubectl -n "$namespace" get pdb "$pdb" -o jsonpath='{.status.disruptionsAllowed}')" == '0' ]]; then
      return 0
    fi
    sleep 0.5
  done
  return 1
}

assert_second_eviction_denied() {
  local pdb="$1" first="$2" second="$3" workload="$4"
  evict_pod "$first"
  wait_for_zero_disruptions "$pdb"
  if evict_pod "$second" >"$temporary/${workload}-eviction.out" 2>"$temporary/${workload}-eviction.err"; then
    echo "$workload PDB allowed two simultaneous voluntary disruptions" >&2
    exit 1
  fi
  grep -qi 'disruption budget' "$temporary/${workload}-eviction.err"
}

cleanup() {
  if [[ -n "$port_forward_pid" ]]; then kill "$port_forward_pid" 2>/dev/null || true; fi
  rm -rf -- "$temporary"
}
trap cleanup EXIT

image_id() {
  # The CRI image list (what kubelet resolves against) keys images by their
  # config digest; podman's .Id is that hash but without the sha256: prefix.
  printf 'sha256:%s\n' "$(podman image inspect "$1" --format '{{.Id}}')"
}

nats_digest="$(image_id localhost/rabbit-jetstream/nats-server:kubernetes-smoke)"
management_digest="$(image_id localhost/rabbit-jetstream/management:kubernetes-smoke)"
operator_digest="$(image_id localhost/rabbit-jetstream/operator:kubernetes-smoke)"
for digest in "$nats_digest" "$management_digest" "$operator_digest"; do
  printf '%s\n' "$digest" | grep -Eq '^sha256:[0-9a-f]{64}$'
done
echo "image digests verified"

cd "/root/rc3-cluster"

for node in $(kubectl get nodes -o jsonpath='{range .items[*]}{.metadata.name}{" "}{end}'); do
  podman exec "$node" ctr -n k8s.io images tag "localhost/rabbit-jetstream/nats-server:kubernetes-smoke" "docker.io/rabbit-jetstream/nats-server@$nats_digest"
  podman exec "$node" ctr -n k8s.io images tag "localhost/rabbit-jetstream/management:kubernetes-smoke" "docker.io/rabbit-jetstream/management@$management_digest"
  podman exec "$node" ctr -n k8s.io images tag "localhost/rabbit-jetstream/operator:kubernetes-smoke" "docker.io/rabbit-jetstream/operator@$operator_digest"
done
echo "node image tags applied"

./tls-fixture --output "$temporary/tls"
test "$(sha256sum "$temporary/tls/server/tls.key" | cut -d' ' -f1)" != "$(sha256sum "$temporary/tls/client/tls.key" | cut -d' ' -f1)"
kubectl create namespace "$namespace"
kubectl -n "$namespace" create secret generic rjs-nats-server-tls \
  --from-file=ca.crt="$temporary/tls/server/ca.crt" --from-file=tls.crt="$temporary/tls/server/tls.crt" --from-file=tls.key="$temporary/tls/server/tls.key"
kubectl -n "$namespace" create secret generic rjs-nats-client-tls \
  --from-file=ca.crt="$temporary/tls/client/ca.crt" --from-file=tls.crt="$temporary/tls/client/tls.crt" --from-file=tls.key="$temporary/tls/client/tls.key"
echo "tls secrets created"

helm_upgrade
echo "helm upgrade done"

kubectl -n "$namespace" rollout status "statefulset/${release}-rabbit-jetstream-nats" --timeout=5m
kubectl -n "$namespace" rollout status "deployment/${release}-rabbit-jetstream-management" --timeout=5m
test "$(kubectl -n "$namespace" get pod -l app.kubernetes.io/component=nats -o jsonpath='{range .items[*]}{.spec.nodeName}{"\n"}{end}' | sort -u | grep -c .)" -eq 3
test "$(kubectl -n "$namespace" get pvc -l app.kubernetes.io/instance="$release" -o jsonpath='{range .items[*]}{.status.phase}{"\n"}{end}' | grep -c '^Bound$')" -eq 3
echo "rollouts, 3-node spread and PVC binding verified"

auth_secret="${release}-rabbit-jetstream-auth"
auth_before="$(kubectl -n "$namespace" get secret "$auth_secret" -o jsonpath='{.data}')"
helm_upgrade
auth_after="$(kubectl -n "$namespace" get secret "$auth_secret" -o jsonpath='{.data}')"
test "$auth_before" = "$auth_after"
admin_token="$(kubectl -n "$namespace" get secret "$auth_secret" -o jsonpath='{.data.admin-token}' | base64 -d)"
test -n "$admin_token"
echo "auth secret stability verified"

management_ip="$(kubectl -n "$namespace" get service "${release}-rabbit-jetstream-management" -o jsonpath='{.spec.clusterIP}')"
kubectl -n "$namespace" run network-allowed --image="$busybox_image" --restart=Never --attach --rm --command -- \
  wget -T 10 -qO- "http://${management_ip}:8223/readyz" | grep -F '"status":"ready"' >/dev/null
denied_namespace="${namespace}-denied"
kubectl create namespace "$denied_namespace"
denied_result="$(kubectl -n "$denied_namespace" run network-denied --image="$busybox_image" --restart=Never --attach --rm --command -- \
  sh -c "if wget -T 5 -qO- http://${management_ip}:8223/readyz; then echo 'management NetworkPolicy allowed an unauthorized cross-namespace Pod' >&2; exit 42; fi; echo network-policy-denied")"
printf '%s\n' "$denied_result" | grep -q '^network-policy-denied$'
echo "management NetworkPolicy verified"

nats_ip="$(kubectl -n "$namespace" get service "${release}-rabbit-jetstream-nats" -o jsonpath='{.spec.clusterIP}')"
kubectl -n "$namespace" run nats-network-allowed --labels='rabbit-jetstream.io/nats-client=true' --image="$busybox_image" --restart=Never --attach --rm --command -- \
  nc -z -w 10 "$nats_ip" 4222
nats_denied_result="$(kubectl -n "$denied_namespace" run nats-network-denied --image="$busybox_image" --restart=Never --attach --rm --command -- \
  sh -c "if nc -z -w 5 ${nats_ip} 4222; then echo 'NATS NetworkPolicy allowed an unauthorized cross-namespace Pod' >&2; exit 42; fi; echo nats-network-policy-denied")"
printf '%s\n' "$nats_denied_result" | grep -q '^nats-network-policy-denied$'
echo "nats NetworkPolicy verified"

assert_second_eviction_denied "${release}-rabbit-jetstream-nats" "${release}-rabbit-jetstream-nats-0" "${release}-rabbit-jetstream-nats-1" nats
kubectl -n "$namespace" rollout status "statefulset/${release}-rabbit-jetstream-nats" --timeout=5m
management_first="$(kubectl -n "$namespace" get pod -l app.kubernetes.io/component=management -o name | sort | sed -n '1p' | cut -d/ -f2)"
management_second="$(kubectl -n "$namespace" get pod -l app.kubernetes.io/component=management -o name | sort | sed -n '2p' | cut -d/ -f2)"
test -n "$management_first" && test -n "$management_second"
assert_second_eviction_denied "${release}-rabbit-jetstream-management" "$management_first" "$management_second" management
kubectl -n "$namespace" rollout status "deployment/${release}-rabbit-jetstream-management" --timeout=5m
echo "PDB disruption budgets verified"

podman run --rm --network host -v "$HOME/.kube:/root/.kube:ro" \
  "$helm_image" test "$release" --namespace "$namespace" --logs --timeout 3m
echo "helm test passed"

kubectl -n "$namespace" port-forward --address 127.0.0.1 "service/${release}-rabbit-jetstream-management" "$local_port:8223" >"$temporary/port-forward.log" 2>&1 &
port_forward_pid=$!
ready='false'
for _ in $(seq 1 30); do
  if curl --fail --silent --show-error "http://127.0.0.1:$local_port/readyz" | grep -F '"status":"ready"' >/dev/null; then
    curl --fail --silent --show-error "http://127.0.0.1:$local_port/admin/" | grep -F '<title>Rabbit JetStream</title>' >/dev/null
    capabilities="$(curl --fail --silent --show-error \
      -H "Authorization: Bearer $admin_token" \
      "http://127.0.0.1:$local_port/api/v1/console/capabilities")"
    printf '%s\n' "$capabilities" | grep -F '"deployment":{"profile":"cluster","source":"configuration"}' >/dev/null
    ready='true'
    break
  fi
  sleep 1
done
if [[ "$ready" != 'true' ]]; then
  cat "$temporary/port-forward.log" >&2
  kubectl -n "$namespace" get pods -o wide >&2
  exit 1
fi
echo "admin UI and cluster profile verified"
printf '%s\n' "$capabilities" > "/root/rc3-cluster/smoke-capabilities.json"

kill "$port_forward_pid" 2>/dev/null || true
wait "$port_forward_pid" 2>/dev/null || true
port_forward_pid=''
podman run --rm --network host -v "$HOME/.kube:/root/.kube:ro" \
  "$helm_image" uninstall "$release" --namespace "$namespace" --wait --timeout 3m
test "$(kubectl -n "$namespace" get pvc -l app.kubernetes.io/instance="$release" -o jsonpath='{range .items[*]}{.status.phase}{"\n"}{end}' | grep -c '^Bound$')" -eq 3
echo "helm uninstall clean, PVCs retained"
echo "RC3 CLUSTER SMOKE: ALL CHECKS PASSED"
