#!/bin/bash

set -e 

if [ "$1" = 'celestia' ]; then
    ./celestia "${NODE_TYPE}" --store.path /celestia-"${NODE_TYPE}" init

    exec ./"$@" "--"
fi

exec "$@"
