#!/usr/bin/env bash
# run-demo.sh — bring up the executor, run three planner scenarios, tear down.
#
# Assumes the sidecar is already running on :8181. If it isn't, run
# `make demo` from the repo root instead, which orchestrates all three.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

echo "── building demo binaries ──"
go build -o /tmp/planner  ./planner
go build -o /tmp/executor ./executor

echo "── starting executor on :8081 ──"
/tmp/executor --listen :8081 --sidecar-url "http://localhost:8181/authorize" &
EXECUTOR_PID=$!
trap 'kill $EXECUTOR_PID 2>/dev/null || true' EXIT
sleep 1

echo
echo "── SCENARIO 1: happy path ──"
/tmp/planner --executor-url "http://localhost:8081/invoke" --scenario happy || true

echo
echo "── SCENARIO 2: scope creep ──"
/tmp/planner --executor-url "http://localhost:8081/invoke" --scenario scope_creep || true

echo
echo "── SCENARIO 3: tampered chain ──"
/tmp/planner --executor-url "http://localhost:8081/invoke" --scenario tampered || true

echo
echo "── demo complete ──"
