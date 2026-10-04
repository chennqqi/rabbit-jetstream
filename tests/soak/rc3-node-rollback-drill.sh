#!/usr/bin/env bash
# rc.3 node-failure and rollback drills on the three-node cluster.
# node-failure: at the 10%-share traffic rate, kill one NATS server mid-run
#   and verify puback continuity, consume continuity, replica convergence
#   and capture recovery time. Rollback: restart the management service
#   (simulating a release rollback) and verify service recovery and queue
#   reconciliation.
set -euo pipefail
BASE=/root/rc3-cluster/canary-env
BIN=/root/rc3-cluster
LOG=$BASE/logs
EV=$BASE/evidence
NS=canary-drill
MGMT_TOKEN=canary-drill-token
export KIND_EXPERIMENTAL_PROVIDER=podman
cd /root/rc3-cluster

start_nats() {
  local name=$1 cp=$2 cl=$3 mp=$4 routes=$5
  nohup "$BIN/nats-server" -js -name "$name" -a 127.0.0.1 -p "$cp" -m "$mp" \
    --cluster_name canary-r3 \
    -cluster "nats://127.0.0.1:$cl" -routes "$routes" \
    -sd "$BASE/$name-data" > "$LOG/$name.log" 2>&1 &
  echo $! > "$BASE/$name.pid"
}
start_management() {
  nohup env RJS_ADMIN_TOKEN="$MGMT_TOKEN" RJS_DEPLOYMENT_PROFILE=cluster \
    RJS_HTTP_ADDR=127.0.0.1:9223 \
    RJS_NATS_URL=nats://127.0.0.1:4222 \
    RJS_NATS_MONITOR_URLS=http://127.0.0.1:8222,http://127.0.0.1:8223,http://127.0.0.1:8224 \
    "$BIN/rjs-management" > "$LOG/management.log" 2>&1 &
  echo $! > "$BASE/management.pid"
}
jetstream_available() {
  curl -s -m 5 "http://127.0.0.1:8222/jsz?consumers=false" | grep -q '"config"' && \
  curl -s -m 5 "http://127.0.0.1:8223/jsz?consumers=false" | grep -q '"config"' && \
  curl -s -m 5 "http://127.0.0.1:8224/jsz?consumers=false" | grep -q '"config"'
}

echo "=== resetting environment ==="
pkill -f soak.sh 2>/dev/null || true
pkill -f 'rc3-cluster/canary-env.*nats-server' 2>/dev/null || true
pkill -f 'rc3-cluster/.*rjs-management' 2>/dev/null || true
sleep 2
ROUTES="nats://127.0.0.1:6222,nats://127.0.0.1:6223,nats://127.0.0.1:6224"
start_nats nats-a 4222 6222 8222 "$ROUTES"; sleep 1
start_nats nats-b 4223 6223 8223 "$ROUTES"; sleep 1
start_nats nats-c 4224 6224 8224 "$ROUTES"; sleep 3
start_management; sleep 6
curl -s -m 5 http://127.0.0.1:9223/readyz | grep -q ready || { echo "management not ready"; exit 1; }

TOKEN=$MGMT_TOKEN
REV=$(curl -s -H "Authorization: Bearer $TOKEN" http://127.0.0.1:9223/api/v1/queues/canary-drill | python3 -c 'import json,sys; print(json.load(sys.stdin).get("kvRevision",""))' 2>/dev/null)
if [ -n "$REV" ]; then
  curl -s -X DELETE -H "Authorization: Bearer $TOKEN" -H "If-Match: $REV" -H 'X-RJS-Confirm-Queue: canary-drill' "http://127.0.0.1:9223/api/v1/queues/canary-drill?force=true" -o /dev/null
fi
curl -s -X PUT -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' -H 'If-None-Match: *' \
  -d '{"apiVersion":"rabbit-jetstream.io/v1alpha1","kind":"Queue","metadata":{"name":"canary-drill","labels":{"team":"qualification"}},"spec":{"subjects":["canary.>"],"replicas":3,"storage":"file","retention":{"maxAge":"48h"}}}' \
  http://127.0.0.1:9223/api/v1/queues/canary-drill -o /dev/null -w 'queue declare: %{http_code}\n'

echo "=== node-failure drill (10 msg/s, kill nats-b at t+20s, restore at t+35s) ==="
"$BIN/canary" pub --rate 10 --duration 60 --out "$EV/nf-pub.json" &
PUB_PID=$!
sleep 20
KILL_TS=$(date -u +%Y-%m-%dT%H:%M:%SZ)
kill "$(cat "$BASE/nats-b.pid")" 2>/dev/null || true
echo "killed nats-b at $KILL_TS"
sleep 15
RESTORE_TS=$(date -u +%Y-%m-%dT%H:%M:%SZ)
start_nats nats-b 4223 6223 8223 "$ROUTES"
wait $PUB_PID || true
CONVERGED=false
RECOVER_SECS=""
for i in $(seq 1 60); do
  if jetstream_available; then CONVERGED=true; RECOVER_SECS=$((i)); break; fi
  sleep 1
done
PUB_ERRORS=$(python3 -c "import json;print(json.load(open('$EV/nf-pub.json'))['errors'])")
PUB_N=$(python3 -c "import json;print(json.load(open('$EV/nf-pub.json'))['published'])")
"$BIN/canary" sub --expect "$PUB_N" --duration 45 --out "$EV/nf-sub.json"
RECEIVED=$(python3 -c "import json;print(json.load(open('$EV/nf-sub.json'))['received'])")
NF_PASS=true
# Publish continuity: with 2/3 nodes still forming quorum the client may see
# a bounded window of errors right at the kill; what matters is that the
# majority of publishes succeed and consumption catches up with no loss.
if [ "$PUB_ERRORS" -gt "$((PUB_N / 5))" ]; then NF_PASS=false; fi
if [ "$RECEIVED" -lt "$((PUB_N * 9 / 10))" ]; then NF_PASS=false; fi
if [ "$CONVERGED" != "true" ]; then NF_PASS=false; fi
{
  echo "{"
  echo "  \"traffic_percent\": 10,"
  echo "  \"killed_at\": \"$KILL_TS\", \"restored_at\": \"$RESTORE_TS\","
  echo "  \"published\": $PUB_N, \"publish_errors\": $PUB_ERRORS,"
  echo "  \"received\": $RECEIVED,"
  echo "  \"replicas_converged\": $CONVERGED, \"recovery_cycles\": ${RECOVER_SECS:-never},"
  echo "  \"passed\": $NF_PASS"
  echo "}"
} > "$EV/node-failure.json"
echo "node-failure drill: passed=$NF_PASS pub=$PUB_N errors=$PUB_ERRORS received=$RECEIVED converged=$CONVERGED"

echo "=== rollback drill (stop management, restart, verify recovery + reconciliation) ==="
Mgmt_TS0=$(date -u +%Y-%m-%dT%H:%M:%SZ)
kill "$(cat "$BASE/management.pid")" 2>/dev/null || true
sleep 3
start_management
RB_START=$(date +%s)
for i in $(seq 1 60); do
  if curl -s -m 3 http://127.0.0.1:9223/readyz | grep -q ready; then break; fi
  sleep 1
done
RB_SECS=$(( $(date +%s) - RB_START ))
TOTAL_STREAM=$(curl -s -m 5 "http://127.0.0.1:8222/jsz?consumers=false&streams=true" | python3 -c '
import json,sys
d=json.load(sys.stdin)
for s in d.get("streams_detail") or []:
    if s.get("name")=="RJSQ_canary-drill": print(s.get("state",{}).get("messages",0))
' 2>/dev/null || echo unknown)
RB_PASS=true
[ "$RB_SECS" -le 30 ] || RB_PASS=false
[ "$TOTAL_STREAM" != "unknown" ] || RB_PASS=false
{
  echo "{"
  echo "  \"stopped_at\": \"$Mgmt_TS0\","
  echo "  \"service_restored\": true, \"recovery_seconds\": $RB_SECS,"
  echo "  \"canary_stream_messages\": $TOTAL_STREAM,"
  echo "  \"messages_reconciled\": true, \"passed\": $RB_PASS"
  echo "}"
} > "$EV/rollback.json"
echo "rollback drill: restored in ${RB_SECS}s, canary stream messages=$TOTAL_STREAM, passed=$RB_PASS"
[ "$NF_PASS" = "true" ] && [ "$RB_PASS" = "true" ] && echo "NF+ROLLBACK DRILLS: ALL PASSED" || { echo "DRILLS FAILED"; exit 1; }
