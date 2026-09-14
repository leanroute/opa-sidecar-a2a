#!/usr/bin/env bash
# In-container version of run-demo.sh. Assumes prebuilt planner + executor
# binaries at /app/planner and /app/executor, and that the sidecar is
# reachable at $SIDECAR_URL (set by docker-compose).

set -euo pipefail

SIDECAR_URL="${SIDECAR_URL:-http://sidecar:8181/authorize}"

echo "── starting executor on :8081 ──"
/app/executor --listen :8081 --sidecar-url "$SIDECAR_URL" &
EXECUTOR_PID=$!
trap 'kill $EXECUTOR_PID 2>/dev/null || true' EXIT
sleep 1

for scenario in happy scope_creep tampered; do
    echo
    echo "── SCENARIO: $scenario ──"
    /app/planner --executor-url "http://localhost:8081/invoke" --scenario "$scenario" || true
done

echo
echo "── demo complete ──"
# Keep the container up briefly so `docker compose logs demo` shows the output.
sleep 5
