#!/usr/bin/env bash
set -euo pipefail
export P2P_NETWORK=${P2P_NETWORK:-devnet}
rpc=http://${CORE_HOST:-validator}:${CORE_RPC_PORT:-26657}
deadline=$((SECONDS + ${STARTUP_TIMEOUT:-120}))
until hash=$(curl --max-time 2 -fsS "$rpc/block?height=1" | jq -er '.result.block_id.hash'); do
  if ((SECONDS >= deadline)); then
    echo 'Timed out waiting for the first validator block.' >&2
    exit 1
  fi
  sleep 1
done
export CELESTIA_CUSTOM="$P2P_NETWORK:$hash"
if [[ ! -f /data/bridge/initialized ]]; then
  mkdir -p /data
  celestia bridge init --node.store /data/bridge --p2p.network "$P2P_NETWORK"
  echo password | cel-key import node-0 /credentials/node-0.key --keyring-backend test --keyring-dir /data/bridge/keys
  printf '%s\n' "$CELESTIA_CUSTOM" >/data/bridge/initialized
fi
[[ $(cat /data/bridge/initialized) == "$CELESTIA_CUSTOM" ]] || {
  echo 'Bridge state belongs to another chain; reset the devnet volumes.' >&2
  exit 1
}
celestia bridge start --node.store /data/bridge --p2p.network "$P2P_NETWORK" \
  --core.ip "${CORE_HOST:-validator}" --core.port "${CORE_GRPC_PORT:-9090}" \
  --rpc.addr 0.0.0.0 --rpc.skip-auth --keyring.keyname node-0 "$@" &
pid=$!
# shellcheck disable=SC2329
cleanup() {
  kill -TERM "$pid" 2>/dev/null || true
  for ((i=0; i<5; i++)); do
    kill -0 "$pid" 2>/dev/null || break
    sleep 1
  done
  kill -KILL "$pid" 2>/dev/null || true
  wait "$pid" || true
}
trap cleanup EXIT
trap 'exit 0' TERM INT
until timeout --kill-after=1 5 bash /opt/devnet/health.sh bridge; do
  if ! kill -0 "$pid" || ((SECONDS >= deadline)); then
    echo 'Bridge failed to synchronize before the startup deadline.' >&2
    exit 1
  fi
  sleep 1
done
wait "$pid"
