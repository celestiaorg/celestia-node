#!/bin/bash

set -e 

if [ "$1" = 'celestia' ]; then
    ./celestia "${NODE_TYPE}" init --p2p.network "${P2P_NETWORK}"

    exec ./"$@" "--"
fi

exec "$@"
