#!/bin/bash

set -e 

if [ "$1" = 'celestia' ]; then
    echo "We are going to initialize the store"
    ./celestia "${NODE_TYPE}" init --keyring.backend "test" --p2p.network "${P2P_NETWORK}"

    exec ./"$@" "--"
else
echo "Relying on user commands"
exec "$@"
fi