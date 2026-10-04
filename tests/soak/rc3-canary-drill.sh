#!/usr/bin/env bash
# rc.3 canary drill on the native linux/amd64 host: a three-node NATS R3
# cluster plus the cluster-profile management service, then five traffic
# stages (1/10/25/50/100 %) with per-stage evidence aligned to the
# release-approval template: jetstream availability, controller activity,
# node counts, reconcile (missing/corrupt/duplicates), DLQ failures,
# management 5xx, publish p99 and throughput.
set -euo pipefail
BASE=/root/rc3-cluster/canary-env
BIN=/root/rc3-cluster
LOG=$BASE/logs
EV=$BASE/evidence
mkdir -p "$LOG" "$EV"
NS=canary-drill
MGMT_TOKEN=canary-drill-token
timeout_cmd() { timeout 30 "$@"; }

start_nats() { # start_nats <name> <client-port> <cluster-port> <monitor-port> <routes>
  local name=$1 cp=$2 cl=$3 mp=$4 routes=$5
  nohup "$BIN/nats-server" -js -name "$name" -a 127.0.0.1 -p "$cp" -m "$mp" \
    --cluster_name canary-r3 \
    -cluster "nats://127.0.0.1:$cl" -routes "$routes" \
    -sd "$BASE/$name-data" > "$LOG/$name.log" 2>&1 &
  echo $! > "$BASE/$name.pid"
}

stop_all() {
  for f in "$BASE"/*.pid; do [ -f "$f" ] && kill "$(cat "$f")" 2>/dev/null || true; done
  pkill -f 'rc3-cluster/canary-env.*nats-server' 2>/dev/null || true
  pkill -f 'rc3-cluster/.*rjs-management' 2>/dev/null || true
  sleep 1
}

start_management() {
  nohup env RJS_ADMIN_TOKEN="$MGMT_TOKEN" RJS_DEPLOYMENT_PROFILE=cluster \
    RJS_HTTP_ADDR=127.0.0.1:9223 \
    RJS_NATS_URL=nats://127.0.0.1:4222 \
    RJS_NATS_MONITOR_URLS=http://127.0.0.1:8222,http://127.0.0.1:8223,http://127.0.0.1:8224 \
    nohup "$BIN/rjs-management" > "$LOG/management.log" 2>&1 &
  echo $! > "$BASE/management.pid"
}

management_5xx() {
  grep -c 'status=5' "$LOG/management.log" 2>/dev/null || echo 0
}

jetstream_available() {
  curl -s -m 5 "http://127.0.0.1:8222/jsz?consumers=false" | grep -q '"config"' && \
  curl -s -m 5 "http://127.0.0.1:8223/jsz?consumers=false" | grep -q '"config"' && \
  curl -s -m 5 "http://127.0.0.1:8224/jsz?consumers=false" | grep -q '"config"'
}

controller_active() {
  curl -s -m 5 http://127.0.0.1:9223/metrics | grep -E '^rjs_controller_leader\{.*\} 1$' | grep -q .
}

expected_nodes() {
  curl -s -m 5 "http://127.0.0.1:8222/jsz?consumers=false" | python3 -c '
import json,sys
d=json.load(sys.stdin)
streams=d.get("streams_detail") or []
r3=[s for s in streams if s.get("config",{}).get("num_replicas",0)>=3]
print(len(r3))' 2>/dev/null || echo 0
}

stop_all
rm -rf "$BASE"/*-data "$BASE"/*.pid
ROUTES="nats://127.0.0.1:6222,nats://127.0.0.1:6223,nats://127.0.0.1:6224"
start_nats nats-a 4222 6222 8222 "$ROUTES"; sleep 1
start_nats nats-b 4223 6223 8223 "$ROUTES"; sleep 1
start_nats nats-c 4224 6224 8224 "$ROUTES"; sleep 3
start_management; sleep 6

curl -s -m 5 http://127.0.0.1:9223/readyz | grep -q '"status":"ready"' || { echo "management not ready"; exit 1; }
jetstream_available || { echo "jetstream cluster not formed"; exit 1; }
echo "cluster formed: 3-node NATS R3 + management cluster profile"

MGMT=http://127.0.0.1:9223/api/v1
# Declare the canary Queue so the controller has declared topology to reconcile.
curl -s -X PUT -H "Authorization: Bearer $MGMT_TOKEN" -H 'Content-Type: application/json' \
  -H 'If-None-Match: *' -d '{"apiVersion":"rabbit-jetstream.io/v1alpha1","kind":"Queue","metadata":{"name":"canary-drill","labels":{"team":"qualification"}},"spec":{"subjects":["canary.>"],"replicas":3,"storage":"file","retention":{"maxAge":"48h"}}}' \
  "$MGMT/queues/canary-drill" -o /dev/null -w 'queue declare: %{http_code}\n'

# Five canary stages. Stage rates approximate escalating traffic shares
# (1/10/25/50/100 %) of the 100 %-stage reference rate of 100 msg/s.
STAGES="1:1:20 10:10:30 25:25:30 50:50:40 100:100:60"
for STAGE in $STAGES; do
  PCT=${STAGE%%:*}; REST=${STAGE#*:}; RATE=${REST%%:*}; SECS=${REST##*:}
  TS=$(date -u +%Y-%m-%dT%H:%M:%SZ)
  echo "=== canary stage $PCT% (rate=$RATE, ${SECS}s) ==="
  "$BIN/canary" pub --rate "$RATE" --duration "$SECS" --out "$EV/canary-$(printf '%03d' $PCT)-pub.json"
  # Drain and reconcile; expect the published count from the stage summary.
  EXPECT=$(python3 -c "import json;print(json.load(open('$EV/canary-$(printf '%03d' $PCT)-pub.json'))['published'])")
  "$BIN/canary" sub --expect "$EXPECT" --duration 45 --out "$EV/canary-$(printf '%03d' $PCT)-sub.json"
  NODES=$(expected_nodes)
  CTRL=$(controller_active && echo true || echo false)
  JSOK=$(jetstream_available && echo true || echo false)
  FAILS=$(curl -s -m 5 http://127.0.0.1:9223/metrics | grep -E '^rjs_dlq_failed_total' | awk '{print $2}' | head -1)
  FAILS=${FAILS:-0}
  ERRS=$(python3 -c "import json;print(json.load(open('$EV/canary-$(printf '%03d' $PCT)-pub.json'))['errors'])")
  P99=$(python3 -c "import json;print(json.load(open('$EV/canary-$(printf '%03d' $PCT)-pub.json'))['p99_ms'])")
  THR=$(python3 -c "import json;print(json.load(open('$EV/canary-$(printf '%03d' $PCT)-pub.json'))['throughput'])")
  MISSING=$(python3 -c "import json;print(json.load(open('$EV/canary-$(printf '%03d' $PCT)-sub.json'))['missing'])")
  CORRUPT=$(python3 -c "import json;print(json.load(open('$EV/canary-$(printf '%03d' $PCT)-sub.json'))['corrupt'])")
  PEND=$(python3 -c "import json;print(json.load(open('$EV/canary-$(printf '%03d' $PCT)-sub.json'))['received'])")
  HTTP5XX=$(management_5xx)
  {
    echo "{"
    echo "  \"stage_percent\": $PCT,"
    echo "  \"started_at\": \"$TS\","
    echo "  \"ended_at\": \"$(date -u +%Y-%m-%dT%H:%M:%SZ)\","
    echo "  \"jetstream_available\": $JSOK,"
    echo "  \"controller_active\": $CTRL,"
    echo "  \"expected_nodes\": 3, \"current_nodes\": $NODES,"
    echo "  \"missing_messages\": $MISSING, \"corrupt_messages\": $CORRUPT, \"received_messages\": $PEND,"
    echo "  \"dlq_transfer_failures\": $FAILS,"
    echo "  \"management_5xx_count\": $HTTP5XX,"
    echo "  \"publish_errors\": $ERRS,"
    echo "  \"publish_p99_millis\": $P99,"
    echo "  \"throughput_per_second\": $THR,"
    echo "  \"pub_evidence\": \"canary-$(printf '%03d' $PCT)-pub.json\","
    echo "  \"sub_evidence\": \"canary-$(printf '%03d' $PCT)-sub.json\""
    echo "}"
  } > "$EV/canary-stage-$(printf '%03d' $PCT).json"
  echo "stage $PCT% evidence: missing=$MISSING corrupt=$CORRUPT pending=$RECEIVED p99=${P99}ms thr=$THR/s js=$JSOK ctrl=$CTRL nodes=$NODES 5xx=$HTTP5XX"
done
echo "CANARY FIVE STAGES COMPLETE"
echo "$EV"
