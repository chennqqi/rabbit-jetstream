#!/bin/bash
# 24h management-plane soak: supervision + evidence capture.
# Component liveness uses pgrep on the process identity (the nohup wrapper
# PID exits immediately, so PID files cannot detect liveness).
set -u
BASE=/home/sandbox/rc3
BIN=$BASE/bin
RUN=$BASE/run
LOG=$BASE/logs
mkdir -p "$RUN" "$LOG"
NATS_RESTARTS=0
MGMT_RESTARTS=0
PUB_RESTARTS=0
START=$(date -u +%Y-%m-%dT%H:%M:%SZ)
echo "$START soak supervisor start revision=$(cat "$BIN/revision.txt")" >> "$LOG/soak-evidence.log"

start_nats() { pkill -x nats-server 2>/dev/null; sleep 1; nohup "$BIN/nats-server" -js -a 127.0.0.1 -p 4222 -m 8222 -sd "$BASE/nats-data" > "$LOG/nats-server.log" 2>&1 & NATS_RESTARTS=$((NATS_RESTARTS+1)); }
start_mgmt() { pkill -f 'bin/rjs-management' 2>/dev/null; sleep 1; env RJS_ADMIN_TOKEN=rc3-soak-token RJS_DEPLOYMENT_PROFILE=standalone RJS_HTTP_ADDR=127.0.0.1:8223 RJS_NATS_URL=nats://127.0.0.1:4222 RJS_NATS_MONITOR_URLS=http://127.0.0.1:8222 nohup "$BIN/rjs-management" > "$LOG/rjs-management.log" 2>&1 & MGMT_RESTARTS=$((MGMT_RESTARTS+1)); }
start_pub()  { pkill -f 'bin/soak-pub' 2>/dev/null; sleep 1; env SOAK_NATS_URL=nats://127.0.0.1:4222 nohup "$BIN/soak-pub" > "$LOG/soak-pub.log" 2>&1 & PUB_RESTARTS=$((PUB_RESTARTS+1)); }

alive_nats() { pgrep -x nats-server > /dev/null; }
alive_mgmt() { pgrep -f 'bin/rjs-management' > /dev/null; }
alive_pub()  { pgrep -f 'bin/soak-pub' > /dev/null; }

start_nats; sleep 3
start_mgmt; sleep 5
start_pub

CYCLE=0
while true; do
  CYCLE=$((CYCLE+1))
  sleep 30
  TS=$(date -u +%Y-%m-%dT%H:%M:%SZ)
  HEALTH=$(curl -s -m 5 -o /dev/null -w '%{http_code}' http://127.0.0.1:8223/healthz || echo conn-refused)
  API=$(curl -s -m 5 -o /dev/null -w '%{http_code}' -H 'Authorization: Bearer rc3-soak-token' http://127.0.0.1:8223/api/v1/queues || echo conn-refused)
  MSGS=$(curl -s -m 5 http://127.0.0.1:8222/jsz?consumers=false 2>/dev/null | grep -o '"messages":[0-9]*' | head -1)
  if ! alive_nats; then echo "$TS nats DOWN (restarts=$NATS_RESTARTS)" >> "$LOG/soak-evidence.log"; start_nats; sleep 3; fi
  if ! alive_mgmt; then echo "$TS mgmt DOWN (restarts=$MGMT_RESTARTS)" >> "$LOG/soak-evidence.log"; start_mgmt; sleep 5; fi
  if ! alive_pub; then echo "$TS pub DOWN (restarts=$PUB_RESTARTS)" >> "$LOG/soak-evidence.log"; start_pub; fi
  echo "$TS cycle=$CYCLE health=$HEALTH queues_api=$API jetstream=$MSGS nats_restarts=$NATS_RESTARTS mgmt_restarts=$MGMT_RESTARTS pub_restarts=$PUB_RESTARTS" >> "$LOG/soak-evidence.log"
done
