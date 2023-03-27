#!/bin/bash

set -e

if [ "$1" = 'celestia' ]; then
    echo "Initializing Celestia Node with command:"
    echo "celestia "${NODE_TYPE}" init --p2p.network "${P2P_NETWORK}""
    echo ""

    celestia "${NODE_TYPE}" init --p2p.network "${P2P_NETWORK}"

    echo ""
fi

echo "Starting Celestia Node with command:"
echo "$@"
echo ""

exec "$@"
