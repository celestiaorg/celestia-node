#! /bin/bash

# Pull the celestia-app image
docker pull ghcr.io/celestiaorg/celestia-app:v1.11.0

# Run the celestia-app container
docker rm -f celestia-app
docker run -d \
  --name celestia-app \
  -p 26656:26656 \
  -p 26657:26657 \
  -p 9090:9090 \
  --network host \
  -v $(pwd)/resources/genesis.sh:/opt/genesis.sh \
  --entrypoint "/bin/bash" \
  ghcr.io/celestiaorg/celestia-app:v1.11.0 -c "sleep infinity"

docker exec celestia-app bash /opt/genesis.sh
docker exec celestia-app celestia-appd start --rpc.laddr=tcp://0.0.0.0:26657 --api.enable --grpc.enable --minimum-gas-prices=0.01utia --home=/home/celestia 1>/dev/null 2>&1 &

# Get the IP address of the celestia-app container
APP_IP=localhost
echo "Celestia App IP: $APP_IP"

# Wait for height 1
echo "Waiting for height 1..."
while true; do
  HEIGHT=$(wget -q -O - http://$APP_IP:26657/status | jq -r .result.sync_info.latest_block_height)
  if [ "$HEIGHT" -ge "1" ]; then
    break
  fi
  sleep 1
done

# Get the chainId
CHAIN_ID=$(wget -q -O - http://$APP_IP:26657/status | jq -r .result.node_info.network)
echo "Chain ID: $CHAIN_ID"

# Get the genesis hash
GENESIS_HASH=$(wget -q -O - http://$APP_IP:26657/block?height=1 | jq -r .result.block_id.hash)
echo "Genesis Hash: $GENESIS_HASH"


# Pull the celestia-node image
docker pull ghcr.io/celestiaorg/celestia-node:v0.14.0

# Run the celestia-node container with shell entry point
# docker rm -f celestia-node
# docker run -d \
#   --name celestia-node \
#   -e CELESTIA_CUSTOM="$CHAIN_ID:$GENESIS_HASH" \
#   -p 2121:2121 \
#   -p 26658:26658 \
#   --entrypoint /bin/sh \
#   ghcr.io/celestiaorg/celestia-node:v0.14.0 -c "sleep infinity"

docker rm -f celestia-node
docker run -d \
  --name celestia-node \
  -e CELESTIA_CUSTOM="$CHAIN_ID:$GENESIS_HASH" \
  -p 2121:2121 \
  -p 26658:26658 \
  --network host \
  ghcr.io/celestiaorg/celestia-node:v0.14.0 \
  /bin/bash -c "celestia bridge init && celestia bridge start" 

# Access the shell of the celestia-node container
# docker exec -it celestia-node /bin/sh

# Get auth token
AUTH_TOKEN=$(docker exec celestia-node celestia bridge auth admin | tail -n1 | tr -cd '[:print:]')
echo "Auth Token: $AUTH_TOKEN"


# run get p2p info with token
docker exec celestia-node celestia p2p info --token ${AUTH_TOKEN} --url http://localhost:26658


