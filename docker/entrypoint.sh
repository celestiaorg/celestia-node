#!/bin/bash

set -e 

if [ "$1" = 'celestia' ]; then
    ./celestia "${NODE_TYPE}" --node.store /celestia-"${NODE_TYPE}" init

    exec ./"$@" "--"
fi

exec "$@"
