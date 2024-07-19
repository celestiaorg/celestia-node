#!/bin/sh
CHAIN_ID="test"
KEYRING_BACKEND="test"
KEY_NAME="node"
INITIAL_TIA_AMOUNT="1000000000000000utia"
CELESTIA_HOME="/home/celestia"

celestia-appd init "${CHAIN_ID}" --home "${CELESTIA_HOME}" --chain-id "${CHAIN_ID}"
# Build genesis file incl account for passed address
celestia-appd keys add ${KEY_NAME} --keyring-backend=${KEYRING_BACKEND} --home "${CELESTIA_HOME}"

# this won't work because some proto types are declared twice and the logs output to stdout (dependency hell involving iavl)
account_address=$(celestia-appd keys show ${KEY_NAME} -a --home "${CELESTIA_HOME}")
celestia-appd add-genesis-account "${account_address}" ${INITIAL_TIA_AMOUNT} --home "${CELESTIA_HOME}"
celestia-appd gentx ${KEY_NAME} 5000000000utia --chain-id ${CHAIN_ID} --home "${CELESTIA_HOME}"

celestia-appd collect-gentxs --home "${CELESTIA_HOME}"