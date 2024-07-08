package testnet

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/celestiaorg/celestia-app/v2/test/e2e/testnet"
	"github.com/celestiaorg/knuu/pkg/knuu"
	"github.com/prometheus/common/expfmt"
)

const (
	rpcPort        = 26658
	p2pPort        = 2121
	prometheusPort = 9090
	otlpRemotePort = 4318
	dockerSrcURL   = "ghcr.io/celestiaorg/celestia-node"
	remoteRootDir  = "/home/celestia"
	txsimRootDir   = "/home/celestia"
)

type Node struct {
	Name     string
	Type     string `json:"type" validate:"oneof=bridge full light"`
	Version  string
	Instance *knuu.Instance

	rpcProxyHost        string
	prometheusProxyHost string
}

func initInstance(instanceName, version, nodeType, chainId, genesisHash string) (*knuu.Instance, error) {
	instance, err := knuu.NewInstance(instanceName)
	if err != nil {
		return nil, fmt.Errorf("error creating instance: '%v'", err)
	}
	err = instance.SetImage(fmt.Sprintf("ghcr.io/celestiaorg/celestia-node:%s", version))
	if err != nil {
		return nil, fmt.Errorf("Error setting image: %v", err)
	}
	err = instance.AddPortTCP(p2pPort)
	if err != nil {
		return nil, fmt.Errorf("Error adding port: %v", err)
	}
	err = instance.AddPortTCP(rpcPort)
	if err != nil {
		return nil, fmt.Errorf("Error adding port: %v", err)
	}
	_, err = instance.ExecuteCommand("celestia", nodeType, "init", "--node.store", remoteRootDir)
	if err != nil {
		return nil, fmt.Errorf("Error executing command: %v", err)
	}
	err = instance.Commit()
	if err != nil {
		return nil, fmt.Errorf("Error committing instance: %v", err)
	}
	err = instance.SetEnvironmentVariable("CELESTIA_CUSTOM", fmt.Sprintf("%s:%s", chainId, genesisHash))
	if err != nil {
		return nil, fmt.Errorf("Error setting environment variable: %v", err)
	}

	err = instance.SetOtelEndpoint(otlpRemotePort)
	if err != nil {
		return nil, fmt.Errorf("Error setting otel endpoint: %v", err)
	}
	err = instance.SetPrometheusExporter(fmt.Sprintf("0.0.0.0:%d", prometheusPort))
	if err != nil {
		return nil, fmt.Errorf("Error setting prometheus exporter: %v", err)
	}
	return instance, nil
}

func CreateBridge(
	executor *knuu.Executor,
	instanceName string,
	version string,
	consensus *knuu.Instance,
	resources testnet.Resources,
) (*Node, error) {
	chainId, err := ChainId(executor, consensus)
	if err != nil {
		return nil, fmt.Errorf("error getting chain ID: %w", err)
	}
	genesisHash, err := GenesisHash(executor, consensus)
	if err != nil {
		return nil, fmt.Errorf("error getting genesis hash: %w", err)
	}
	consensusIP, err := consensus.GetIP()
	if err != nil {
		return nil, fmt.Errorf("error getting IP: %w", err)
	}

	bridge, err := initInstance(instanceName, version, "bridge", chainId, genesisHash)
	if err != nil {
		return nil, fmt.Errorf("error creating instance: %w", err)
	}

	err = bridge.SetMemory(resources.MemoryRequest, resources.MemoryLimit)
	if err != nil {
		return nil, fmt.Errorf("error setting memory: %w", err)
	}

	err = bridge.SetCPU(resources.CPU)
	if err != nil {
		return nil, fmt.Errorf("error setting CPU: %w", err)
	}

	err = bridge.SetCommand(
		"celestia",
		"bridge",
		"start",
		"--node.store", remoteRootDir,
		"--core.ip", consensusIP,
		"--rpc.addr", "0.0.0.0",
		"--metrics",
		"--metrics.endpoint", fmt.Sprintf("localhost:%d", otlpRemotePort),
		"--metrics.tls=false",
	)
	if err != nil {
		return nil, fmt.Errorf("error setting command: %w", err)
	}

	testNode := &Node{
		Name:     instanceName,
		Type:     "bridge",
		Version:  version,
		Instance: bridge,
	}

	return testNode, nil
}

func CreateAndStartBridge(
	executor *knuu.Executor,
	instanceName string,
	version string,
	consensus *knuu.Instance,
	resources testnet.Resources,
) (*Node, error) {
	bridge, err := CreateBridge(executor, instanceName, version, consensus, resources)
	if err != nil {
		return nil, fmt.Errorf("error creating bridge: %w", err)
	}

	if err := bridge.Instance.Start(); err != nil {
		return nil, fmt.Errorf("error starting bridge: %w", err)
	}

	err, rpcProxyHost := bridge.Instance.AddHost(rpcPort)
	if err != nil {
		return nil, fmt.Errorf("error adding host: %w", err)
	}
	err, prometheusProxyHost := bridge.Instance.AddHost(prometheusPort)
	if err != nil {
		return nil, fmt.Errorf("error adding host: %w", err)
	}

	bridge.rpcProxyHost = rpcProxyHost
	bridge.prometheusProxyHost = prometheusProxyHost

	return bridge, nil
}

func CreateNode(
	executor *knuu.Executor,
	instanceName string,
	version string,
	nodeType string,
	consensus *knuu.Instance,
	trustedNode *Node,
	resources testnet.Resources,
) (*Node, error) {
	chainId, err := ChainId(executor, consensus)
	if err != nil {
		return nil, fmt.Errorf("error getting chain ID: %w", err)
	}
	genesisHash, err := GenesisHash(executor, consensus)
	if err != nil {
		return nil, fmt.Errorf("error getting genesis hash: %w", err)
	}

	node, err := initInstance(instanceName, version, nodeType, chainId, genesisHash)
	if err != nil {
		return nil, fmt.Errorf("error creating instance: %w", err)
	}

	p2pInfoNode, err := trustedNode.Instance.ExecuteCommand("celestia", "p2p", "info", "--node.store", remoteRootDir)
	if err != nil {
		return nil, fmt.Errorf("error getting p2p info: %w", err)
	}

	bridgeIP, err := trustedNode.Instance.GetIP()
	if err != nil {
		return nil, fmt.Errorf("error getting IP: %w", err)
	}
	bridgeID, err := iDFromP2PInfo(p2pInfoNode)
	if err != nil {
		return nil, fmt.Errorf("error getting ID: %w", err)
	}
	trustedPeers := fmt.Sprintf("/ip4/%s/tcp/2121/p2p/%s", bridgeIP, bridgeID)

	err = node.SetMemory(resources.MemoryRequest, resources.MemoryLimit)
	if err != nil {
		return nil, fmt.Errorf("error setting memory: %w", err)
	}

	err = node.SetCPU(resources.CPU)
	if err != nil {
		return nil, fmt.Errorf("error setting CPU: %w", err)
	}

	err = node.SetCommand(
		"celestia",
		nodeType, "start",
		"--node.store", remoteRootDir,
		"--headers.trusted-peers", trustedPeers,
		"--metrics",
		"--metrics.endpoint", fmt.Sprintf("localhost:%d", otlpRemotePort),
		"--metrics.tls=false",
	)
	if err != nil {
		return nil, fmt.Errorf("error setting command: %w", err)
	}

	testNode := &Node{
		Name:     instanceName,
		Type:     nodeType,
		Version:  version,
		Instance: node,
	}

	return testNode, nil
}

func CreateAndStartNode(
	executor *knuu.Executor,
	instanceName string,
	version string,
	nodeType string,
	consensus *knuu.Instance,
	trustedNode *Node,
	resources testnet.Resources,
) (*Node, error) {
	node, err := CreateNode(executor, instanceName, version, nodeType, consensus, trustedNode, resources)
	if err != nil {
		return nil, fmt.Errorf("error creating node: %w", err)
	}

	if err := node.Instance.Start(); err != nil {
		return nil, fmt.Errorf("error starting node: %w", err)
	}

	err, rpcProxyHost := node.Instance.AddHost(rpcPort)
	if err != nil {
		return nil, fmt.Errorf("error adding host: %w", err)
	}
	err, prometheusProxyHost := node.Instance.AddHost(prometheusPort)
	if err != nil {
		return nil, fmt.Errorf("error adding host: %w", err)
	}

	node.rpcProxyHost = rpcProxyHost
	node.prometheusProxyHost = prometheusProxyHost

	return node, nil
}

// GetMetric returns a metric from the node
func (n *Node) GetMetric(metricName string) (float64, error) {
	host := n.AddressPrometheus()

	resp, err := http.Get(fmt.Sprintf("%s/metrics", host))
	if err != nil {
		return 0, fmt.Errorf("failed to fetch Prometheus data: %v", err)
	}
	defer resp.Body.Close()

	parser := expfmt.TextParser{}
	metricFamilies, err := parser.TextToMetricFamilies(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("failed to parse Prometheus metrics: %v", err)
	}

	var metricValue float64
	found := false

	if metricFamily, ok := metricFamilies[metricName]; ok {
		for _, metric := range metricFamily.GetMetric() {
			switch {
			case metric.Counter != nil:
				metricValue = metric.Counter.GetValue()
				found = true
			case metric.Gauge != nil:
				metricValue = metric.Gauge.GetValue()
				found = true
			case metric.Untyped != nil:
				metricValue = metric.Untyped.GetValue()
				found = true
			case metric.Summary != nil:
				metricValue = metric.Summary.GetSampleSum()
				found = true
			case metric.Histogram != nil:
				metricValue = metric.Histogram.GetSampleSum()
				found = true
			}
		}
	}

	if !found {
		return 0, fmt.Errorf("metric not found")
	}

	return metricValue, nil
}

type JSONRPCError struct {
	Code    int
	Message string
	Data    string
}

func (e *JSONRPCError) Error() string {
	return fmt.Sprintf("JSONRPC Error - Code: %d, Message: %s, Data: %s", e.Code, e.Message, e.Data)
}

// getStatus returns the status of the node
func getStatus(executor *knuu.Executor, app *knuu.Instance) (string, error) {
	nodeIP, err := app.GetIP()
	if err != nil {
		return "", fmt.Errorf("error getting node ip: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	status, err := executor.ExecuteCommandWithContext(ctx, "wget", "-q", "-O", "-", fmt.Sprintf("%s:26657/status", nodeIP))
	if err != nil {
		return "", fmt.Errorf("error executing command: %w", err)
	}
	return status, nil
}

func NodeIdFromNode(executor *knuu.Executor, node *knuu.Instance) (string, error) {
	status, err := getStatus(executor, node)
	if err != nil {
		return "", fmt.Errorf("error getting status: %v", err)
	}

	id, err := nodeIdFromStatus(status)
	if err != nil {
		return "", fmt.Errorf("error getting node id: %v", err)
	}
	return id, nil
}

func GetHeight(executor *knuu.Executor, app *knuu.Instance) (int64, error) {
	status, err := getStatus(executor, app)
	if err != nil {
		return 0, fmt.Errorf("error getting status: %v", err)
	}
	blockHeight, err := latestBlockHeightFromStatus(status)
	if err != nil {
		return 0, fmt.Errorf("error getting block height: %w", err)
	}
	return blockHeight, nil
}

func WaitForHeight(executor *knuu.Executor, app *knuu.Instance, height int64) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	return WaitForHeightWithContext(ctx, executor, app, height)
}

func WaitForHeightWithContext(ctx context.Context, executor *knuu.Executor, app *knuu.Instance, height int64) error {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			if ctx.Err() != nil {
				return fmt.Errorf("operation canceled: %v", ctx.Err())
			}
			return nil
		case <-ticker.C:
			status, err := getStatus(executor, app)
			if err != nil {
				return fmt.Errorf("error getting status: %v", err)
			}

			blockHeight, err := latestBlockHeightFromStatus(status)
			if err != nil {
				if _, ok := err.(*JSONRPCError); ok {
					// Retry if it's a temporary API error
					continue
				}
				return fmt.Errorf("error getting block height: %w", err)
			}

			if blockHeight >= height {
				return nil
			}
		}
	}
}

func ChainId(executor *knuu.Executor, app *knuu.Instance) (string, error) {
	status, err := getStatus(executor, app)
	if err != nil {
		return "", fmt.Errorf("error getting status: %v", err)
	}
	chainId, err := chainIdFromStatus(status)
	if err != nil {
		return "", fmt.Errorf("error getting chain id: %w", err)
	}
	return chainId, nil
}

func GenesisHash(executor *knuu.Executor, app *knuu.Instance) (string, error) {
	appIP, err := app.GetIP()
	if err != nil {
		return "", fmt.Errorf("error getting app ip: %w", err)
	}
	block, err := executor.ExecuteCommand("wget", "-q", "-O", "-", fmt.Sprintf("%s:26657/block?height=1", appIP))
	if err != nil {
		return "", fmt.Errorf("error getting block: %v", err)
	}
	genesisHash, err := hashFromBlock(block)
	if err != nil {
		return "", fmt.Errorf("error getting hash from block: %v", err)
	}
	return genesisHash, nil
}

func GetPersistentPeers(executor *knuu.Executor, apps []*knuu.Instance) (string, error) {
	var persistentPeers string
	for _, app := range apps {
		validatorIP, err := app.GetIP()
		if err != nil {
			return "", fmt.Errorf("error getting validator IP: %v", err)
		}
		id, err := NodeIdFromNode(executor, app)
		if err != nil {
			return "", fmt.Errorf("error getting node id: %v", err)
		}
		persistentPeers += id + "@" + validatorIP + ":26656" + ","
	}
	return persistentPeers[:len(persistentPeers)-1], nil
}

func nodeIdFromStatus(status string) (string, error) {
	var result map[string]interface{}
	err := json.Unmarshal([]byte(status), &result)
	if err != nil {
		return "", fmt.Errorf("error unmarshalling status: %w", err)
	}

	if errorField, ok := result["error"]; ok {
		errorData, ok := errorField.(map[string]interface{})
		if !ok {
			return "", fmt.Errorf("error field exists but is not a map[string]interface{}")
		}
		jsonError := &JSONRPCError{}
		if errorCode, ok := errorData["code"].(float64); ok {
			jsonError.Code = int(errorCode)
		}
		if errorMessage, ok := errorData["message"].(string); ok {
			jsonError.Message = errorMessage
		}
		if errorData, ok := errorData["data"].(string); ok {
			jsonError.Data = errorData
		}
		return "", jsonError
	}

	resultData, ok := result["result"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("error getting result from status")
	}
	nodeInfo, ok := resultData["node_info"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("error getting node info from status")
	}
	id, ok := nodeInfo["id"].(string)
	if !ok {
		return "", fmt.Errorf("error getting id from node info")
	}
	return id, nil
}

func latestBlockHeightFromStatus(status string) (int64, error) {
	var result map[string]interface{}
	err := json.Unmarshal([]byte(status), &result)
	if err != nil {
		return 0, fmt.Errorf("error unmarshalling status: %w", err)
	}

	if errorField, ok := result["error"]; ok {
		errorData, ok := errorField.(map[string]interface{})
		if !ok {
			return 0, fmt.Errorf("error field exists but is not a map[string]interface{}")
		}
		jsonError := &JSONRPCError{}
		if errorCode, ok := errorData["code"].(float64); ok {
			jsonError.Code = int(errorCode)
		}
		if errorMessage, ok := errorData["message"].(string); ok {
			jsonError.Message = errorMessage
		}
		if errorData, ok := errorData["data"].(string); ok {
			jsonError.Data = errorData
		}
		return 0, jsonError
	}

	resultData, ok := result["result"].(map[string]interface{})
	if !ok {
		return 0, fmt.Errorf("error getting result from status")
	}
	syncInfo, ok := resultData["sync_info"].(map[string]interface{})
	if !ok {
		return 0, fmt.Errorf("error getting sync info from status")
	}
	latestBlockHeight, ok := syncInfo["latest_block_height"].(string)
	if !ok {
		return 0, fmt.Errorf("error getting latest block height from sync info")
	}
	latestBlockHeightInt, err := strconv.ParseInt(latestBlockHeight, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("error converting latest block height to int: %w", err)
	}
	return latestBlockHeightInt, nil
}

func chainIdFromStatus(status string) (string, error) {
	var result map[string]interface{}
	err := json.Unmarshal([]byte(status), &result)
	if err != nil {
		return "", fmt.Errorf("error unmarshalling status: %w", err)
	}
	resultData, ok := result["result"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("error getting result from status")
	}
	nodeInfo, ok := resultData["node_info"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("error getting node info from status")
	}
	chainId, ok := nodeInfo["network"].(string)
	if !ok {
		return "", fmt.Errorf("error getting network from node info")
	}
	return chainId, nil
}

func hashFromBlock(block string) (string, error) {
	var result map[string]interface{}
	err := json.Unmarshal([]byte(block), &result)
	if err != nil {
		return "", fmt.Errorf("error unmarshalling block: %w", err)
	}
	resultData, ok := result["result"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("error getting result from block")
	}
	blockId, ok := resultData["block_id"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("error getting block id from block")
	}
	blockHash, ok := blockId["hash"].(string)
	if !ok {
		return "", fmt.Errorf("error getting hash from block id")
	}
	return blockHash, nil
}

// AddressRPC returns an RPC endpoint address for the node.
// This returns the proxy host that can be used to communicate with the node
func (n Node) AddressRPC() string {
	return n.rpcProxyHost
}

// AddressGRPC returns the GRPC endpoint address for the node.
func (n Node) AddressPrometheus() string {
	return n.prometheusProxyHost
}
