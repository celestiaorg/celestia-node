#!/usr/bin/env bash

CHAIN_ID="test"
APP_PATH="${HOME}/.celestia-app"
NODE_PATH="${HOME}/.celestia-bridge-test/"
CELESTIA_VERSION=$(celestia version 2>&1)

echo "node_path: ${NODE_PATH}"
echo "celestia version: ${CELESTIA_VERSION}"
echo ""

echo "Deleting $NODE_PATH..."
rm -r "$NODE_PATH"

echo "Creating $NODE_PATH/keys..."
mkdir -p $NODE_PATH/keys

echo "Copying keys..."
cp -r $APP_PATH/keyring-test/ $NODE_PATH/keys/keyring-test/

# Try to get the genesis hash. Usually first request returns an empty string (port is not open, curl fails), later attempts
# returns "null" if block was not yet produced.
GENESIS=
CNT=0
MAX=30
while [ "${#GENESIS}" -le 4 -a $CNT -ne $MAX ]; do
	GENESIS=$(curl -s http://127.0.0.1:26657/block?height=1 | jq '.result.block_id.hash' | tr -d '"')
	((CNT++))
	sleep 1
done

export CELESTIA_CUSTOM=test:$GENESIS
echo "$CELESTIA_CUSTOM"

celestia bridge init --node.store $NODE_PATH
celestia bridge start \
  --node.store $NODE_PATH --gateway \
  --core.ip 127.0.0.1 \
  --keyring.keyname validator \
  --gateway.addr 0.0.0.0 \
  --rpc.addr 0.0.0.0
