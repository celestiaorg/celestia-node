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

echo "Starting Celestia Node with command:"
echo "$@"
echo ""

exec "$@"
