#!/bin/bash

# Define namespace variable
NAMESPACE="moji-test-node"

# Create a Kubernetes namespace for the deployment
# kubectl delete namespace $NAMESPACE
kubectl create namespace $NAMESPACE

# Create a ConfigMap for the genesis.sh script
kubectl create configmap genesis-script --namespace=$NAMESPACE --from-file=resources/genesis.sh

# Create a Kubernetes deployment for the celestia-app
kubectl apply -f - <<EOF
apiVersion: apps/v1
kind: Deployment
metadata:
  name: celestia-app
  namespace: $NAMESPACE
spec:
  replicas: 1
  selector:
    matchLabels:
      app: celestia-app
  template:
    metadata:
      labels:
        app: celestia-app
    spec:
      containers:
      - name: celestia-app
        image: ghcr.io/celestiaorg/celestia-app:v1.11.0
        command: ["/bin/bash", "-c", "sleep infinity"]
        ports:
        - containerPort: 26656
        - containerPort: 26657
        - containerPort: 9090
        volumeMounts:
        - name: genesis-script
          mountPath: /opt/genesis.sh
          subPath: genesis.sh
      volumes:
      - name: genesis-script
        configMap:
          name: genesis-script
EOF

# Wait for the celestia-app pod to be ready
kubectl wait --namespace=$NAMESPACE --for=condition=ready pod -l app=celestia-app

# Execute the genesis.sh script inside the celestia-app container
POD_NAME=$(kubectl get pods --namespace=$NAMESPACE -l app=celestia-app -o jsonpath="{.items[0].metadata.name}")
kubectl exec --namespace=$NAMESPACE $POD_NAME -- bash /opt/genesis.sh

# Start the celestia-app service
kubectl exec --namespace=$NAMESPACE $POD_NAME -- celestia-appd start --rpc.laddr=tcp://0.0.0.0:26657 --api.enable --grpc.enable --minimum-gas-prices=0.01utia --home=/home/celestia 1>/dev/null 2>&1 &

# Expose the celestia-app service
kubectl expose pod $POD_NAME --namespace=$NAMESPACE --type=ClusterIP --name=celestia-app-service --port=26656 --target-port=26656
kubectl expose pod $POD_NAME --namespace=$NAMESPACE --type=ClusterIP --name=celestia-app-rpc --port=26657 --target-port=26657
kubectl expose pod $POD_NAME --namespace=$NAMESPACE --type=ClusterIP --name=celestia-app-grpc --port=9090 --target-port=9090

# Get the IP address of the celestia-app pod
APP_IP=$(kubectl get pod $POD_NAME --namespace=$NAMESPACE -o jsonpath="{.status.podIP}")
echo "APP IP: $APP_IP"

# Wait for height 1
echo "Waiting for height 1..."
while true; do
  HEIGHT=$(kubectl exec --namespace=$NAMESPACE $POD_NAME -- wget -q -O - http://$APP_IP:26657/status | jq -r .result.sync_info.latest_block_height)
  if [ "$HEIGHT" -ge "1" ]; then
    break
  fi
  sleep 1
done

# Get the chainId
CHAIN_ID=$(kubectl exec --namespace=$NAMESPACE $POD_NAME -- wget -q -O - http://$APP_IP:26657/status | jq -r .result.node_info.network)
echo "Chain ID: $CHAIN_ID"

# Get the genesis hash
GENESIS_HASH=$(kubectl exec --namespace=$NAMESPACE $POD_NAME -- wget -q -O - http://$APP_IP:26657/block?height=1 | jq -r .result.block_id.hash)
echo "Genesis Hash: $GENESIS_HASH"

# Create a Kubernetes deployment for the celestia-node
kubectl apply -f - <<EOF
apiVersion: apps/v1
kind: Deployment
metadata:
  name: celestia-node
  namespace: $NAMESPACE
spec:
  replicas: 1
  selector:
    matchLabels:
      app: celestia-node
  template:
    metadata:
      labels:
        app: celestia-node
    spec:
      containers:
      - name: celestia-node
        image: ghcr.io/celestiaorg/celestia-node:v0.14.0
        env:
        - name: CELESTIA_CUSTOM
          value: "$CHAIN_ID:$GENESIS_HASH"
        command: ["/bin/bash", "-c", "celestia bridge init && celestia bridge start --core.ip ${APP_IP} --rpc.addr 0.0.0.0 --rpc.skip-auth"]
        ports:
        - containerPort: 2121
        - containerPort: 26658
EOF


# Wait for the celestia-node pod to be ready
kubectl wait --namespace=$NAMESPACE --for=condition=ready pod -l app=celestia-node

# Expose the celestia-node service
NODE_POD_NAME=$(kubectl get pods --namespace=$NAMESPACE -l app=celestia-node -o jsonpath="{.items[0].metadata.name}")
kubectl expose pod $NODE_POD_NAME --namespace=$NAMESPACE --type=ClusterIP --name=celestia-node-service --port=2121 --target-port=2121
kubectl expose pod $NODE_POD_NAME --namespace=$NAMESPACE --type=ClusterIP --name=celestia-node-rpc --port=26658 --target-port=26658

# kubectl exec -it --namespace=$NAMESPACE $NODE_POD_NAME -- bash
# exit 0


# Get the IP address of the celestia-node pod
NODE_IP=$(kubectl get pod $NODE_POD_NAME --namespace=$NAMESPACE -o jsonpath="{.status.podIP}")
echo "Node IP: $NODE_IP"

# Get IP of celestia-node-rpc service
NODE_RPC_IP=$(kubectl get svc celestia-node-rpc --namespace=$NAMESPACE -o jsonpath="{.spec.clusterIP}")
echo "Node RPC IP: $NODE_RPC_IP"

# Get auth token
AUTH_TOKEN=$(kubectl exec --namespace=$NAMESPACE $NODE_POD_NAME -- celestia bridge auth admin | tail -n1 | tr -cd '[:print:]')
echo "Auth Token: $AUTH_TOKEN"


# Create a Kubernetes deployment for the celestia-light-node
kubectl apply -f - <<EOF
apiVersion: apps/v1
kind: Deployment
metadata:
  name: celestia-light-node
  namespace: $NAMESPACE
spec:
  replicas: 1
  selector:
    matchLabels:
      app: celestia-light-node
  template:
    metadata:
      labels:
        app: celestia-light-node
    spec:
      containers:
      - name: celestia-light-node
        image: ghcr.io/celestiaorg/celestia-node:v0.14.0
        env:
        - name: CELESTIA_CUSTOM
          value: "$CHAIN_ID:$GENESIS_HASH"
        command: ["/bin/sh", "-c", "sleep infinity"]
        ports:
        - containerPort: 2121
        - containerPort: 26658
EOF



# Wait for the celestia-light-node pod to be ready
kubectl wait --namespace=$NAMESPACE --for=condition=ready pod -l app=celestia-light-node

LIGHT_NODE_POD_NAME=$(kubectl get pods --namespace=$NAMESPACE -l app=celestia-light-node -o jsonpath="{.items[0].metadata.name}")

# Run get p2p info with token
kubectl exec --namespace=$NAMESPACE $LIGHT_NODE_POD_NAME -- celestia p2p info --token ${AUTH_TOKEN} --url http://${NODE_IP}:26658

kubectl exec -it --namespace=$NAMESPACE $LIGHT_NODE_POD_NAME -- bash

curl -X GET \
 -H "Authorization: Bearer PLACEHOLDER" \
 --data '{ "id": 1,  "jsonrpc": "2.0",  "method": "das.SamplingStats",  "params": []}' http://${NODE_IP}:26658
