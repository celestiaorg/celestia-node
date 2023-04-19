#!/bin/bash

set -e

if [ "$1" = 'celestia' ]; then
    echo "Initializing Celestia Node with command:"

    if [[ -n "$NODE_STORE" ]]; then
        echo "celestia "${NODE_TYPE}" init --p2p.network "${P2P_NETWORK}" --node.store "${NODE_STORE}""
        celestia "${NODE_TYPE}" init --p2p.network "${P2P_NETWORK}" --node.store "${NODE_STORE}"
    else
        echo "celestia "${NODE_TYPE}" init --p2p.network "${P2P_NETWORK}""
        celestia "${NODE_TYPE}" init --p2p.network "${P2P_NETWORK}"
    fi

    echo ""
    echo ""
fi

echo "Starting Celestia Node with command:"
echo "$@"
echo ""

exec "$@"
