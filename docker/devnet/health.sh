#!/usr/bin/env bash
set -euo pipefail
height=$(curl --max-time 2 -fsS "http://${CORE_HOST:-validator}:${CORE_RPC_PORT:-26657}/status" | jq -er '.result.sync_info.latest_block_height | tonumber')
((height > 2))
celestia header get-by-height "$((height - 1))" --url http://127.0.0.1:26658 --token skip-auth |
  jq -e --argjson height "$((height - 1))" '.result.header.height | tonumber == $height' >/dev/null
