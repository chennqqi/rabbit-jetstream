#!/usr/bin/env bash
set -euo pipefail

minimum="${RJS_COVERAGE_MIN:-80.0}"
profile="$(mktemp)"
trap 'rm -f "$profile"' EXIT

go test -coverprofile="$profile" ./...
coverage="$(go tool cover -func="$profile" | awk '/^total:/ {gsub(/%/, "", $3); print $3}')"
awk -v actual="$coverage" -v minimum="$minimum" 'BEGIN {
  printf "total coverage: %.1f%% (required: %.1f%%)\n", actual, minimum
  if (actual + 0 < minimum + 0) exit 1
}'
