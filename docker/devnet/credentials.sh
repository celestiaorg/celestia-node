#!/usr/bin/env bash
set -euo pipefail

mkdir -p /credentials /tmp/devnet-keyring
for i in {0..10}; do
  name=node-$i
  [[ $i != 10 ]] || name=validator-0
  printf '%064x\n' "$((i + 1))" >"/credentials/$name.plaintext-key"
  cel-key import-hex "$name" "$(cat "/credentials/$name.plaintext-key")" \
    --node.type bridge --p2p.network devnet --keyring-dir /tmp/devnet-keyring --keyring-backend test \
    >/dev/null 2>&1
  echo password | cel-key export "$name" \
    --node.type bridge --p2p.network devnet --keyring-dir /tmp/devnet-keyring --keyring-backend test \
    >/dev/null 2>"/credentials/$name.key"
  cel-key show "$name" -a \
    --node.type bridge --p2p.network devnet --keyring-dir /tmp/devnet-keyring --keyring-backend test \
    >"/credentials/$name.addr"
done
cel-key show validator-0 --bech val -a \
  --node.type bridge --p2p.network devnet --keyring-dir /tmp/devnet-keyring --keyring-backend test \
  >/credentials/validator-0.valaddr
