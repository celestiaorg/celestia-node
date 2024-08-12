#!/usr/bin/env bash

# This script can be used in conjunction with
# https://github.com/celestiaorg/celestia-app/blob/main/scripts/single-node.sh
# to start a local devnet with one celestia-app validator and one celestia
# bridge node.

# Stop script execution if an error is encountered
set -o errexit
# Stop script execution if an undefined variable is used
set -o nounset

if ! [ -x "$(command -v celestia)" ]
then
    echo "celestia could not be found. Please install the celestia binary using 'make install' and make sure the PATH contains the directory where the binary exists. By default, go will install the binary under '~/go/bin'"
    exit 1
fi

CHAIN_ID="test"
KEY_NAME="validator"
APP_PATH="${HOME}/.celestia-app"
NODE_PATH="${HOME}/.celestia-bridge-test/"
CELESTIA_VERSION=$(celestia version 2>&1)

echo "celestia path: ${NODE_PATH}"
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
echo "celestia custom: $CELESTIA_CUSTOM"

echo "Initializing celestia bridge..."
celestia bridge init --node.store $NODE_PATH

echo "Starting celestia bridge..."
celestia bridge start \
  --node.store $NODE_PATH \
  --gateway \
  --core.ip 127.0.0.1 \
  --keyring.keyname $KEY_NAME \
  --gateway.addr 0.0.0.0 \
  --rpc.addr 0.0.0.0
