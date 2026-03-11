#!/bin/bash

set -e

if [ "$1" = 'celestia' ]; then
    echo "Initializing Celestia Node with command:"
    COMMAND="celestia "${NODE_TYPE}" init --p2p.network "${P2P_NETWORK}" --rpc.addr="0.0.0.0""
    if [[ -n "$NODE_STORE" ]]; then
        COMMAND=${COMMAND}" --node.store "${NODE_STORE}""
        echo $COMMAND
        $COMMAND
    else
        echo $COMMAND
        $COMMAND
    fi

    echo ""
    echo ""
fi

# Apply RDA_EXPECTED_NODES env var to config.toml before starting the node.
# This lets operators set grid size without rebuilding the image.
if [[ -n "$RDA_EXPECTED_NODES" && "$1" = 'celestia' ]]; then
    CONFIG_STORE="${NODE_STORE:-$HOME/.celestia-${NODE_TYPE}-private}"
    CONFIG_FILE="${CONFIG_STORE}/config.toml"
    if [[ -f "$CONFIG_FILE" ]]; then
        if grep -q "RDAExpectedNodeCount" "$CONFIG_FILE"; then
            sed -i "s/RDAExpectedNodeCount = [0-9]*/RDAExpectedNodeCount = ${RDA_EXPECTED_NODES}/" "$CONFIG_FILE"
        else
            # Field not present (e.g. legacy config) – append inside the file
            echo "RDAExpectedNodeCount = ${RDA_EXPECTED_NODES}" >> "$CONFIG_FILE"
        fi
        echo "RDA: ExpectedNodeCount set to ${RDA_EXPECTED_NODES}"
    fi
fi

echo "Starting Celestia Node with command:"
echo "$@"
echo ""

exec "$@"
