package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"time"

	"github.com/celestiaorg/knuu/pkg/instance"
	"github.com/celestiaorg/knuu/pkg/knuu"
	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/celestiaorg/celestia-node/api/rpc/client"
	clientbuilder "github.com/celestiaorg/celestia-openrpc/builder"
	"github.com/celestiaorg/celestia-openrpc/types/das"
)

const (
	appImg         = "ghcr.io/celestiaorg/celestia-app:v1.11.0"
	appName        = "consensus"
	appPortRPC     = 26657
	appPortP2P     = 26656
	appPortGRPC    = 9090
	appHomeDir     = "/home/celestia"
	appMinGasPrice = "0.01utia"
	genesisSh      = "resources/genesis.sh"
	genesisShMap   = "/opt/genesis.sh"
	fileOwner      = "0:0"

	executorName  = "executor"
	executorImage = "docker.io/nicolaka/netshoot:latest"
	sleepCommand  = "sleep"
	infinityArg   = "infinity"

	nodeImage       = "ghcr.io/celestiaorg/celestia-node:v0.14.0"
	nodePort        = 2121
	nodePortGRPC    = 26658
	nodeEnvCustom   = "CELESTIA_CUSTOM"
	nodeStoreFlag   = "--node.store"
	nodeHomeDir     = "/home/celestia"
	nodeTypeBridge  = "bridge"
	nodeTypeFull    = "full"
	nodeTypeLight   = "light"
	celestiaCommand = "celestia"
)

var (
	appArgs = []string{"start", fmt.Sprintf("--rpc.laddr=tcp://0.0.0.0:%d", appPortRPC),
		"--api.enable", "--grpc.enable",
		"--minimum-gas-prices", appMinGasPrice, // we need this so it does not fail to start
		"--home", appHomeDir,
	}

	executorMemoryLimit = resource.MustParse("100Mi")
	executorCpuLimit    = resource.MustParse("100m")
)

type JSONRPCError struct {
	Code    int
	Message string
	Data    string
}

func (e *JSONRPCError) Error() string {
	return fmt.Sprintf("code: %d, message: %s, data: %s", e.Code, e.Message, e.Data)
}

func createAndStartApp(ctx context.Context, kn *knuu.Knuu) (*instance.Instance, error) {
	app, err := kn.NewInstance(appName)
	if err != nil {
		return nil, err
	}

	if err := app.SetImage(ctx, appImg); err != nil {
		return nil, err
	}
	for _, port := range []int{appPortP2P, appPortRPC, appPortGRPC} {
		if err := app.AddPortTCP(port); err != nil {
			return nil, err
		}
	}

	if err := app.AddFile(genesisSh, genesisShMap, fileOwner); err != nil {
		return nil, err
	}

	if _, err := app.ExecuteCommand(ctx, "/bin/sh", genesisShMap); err != nil {
		return nil, err
	}
	if err := app.SetArgs(appArgs...); err != nil {
		return nil, err
	}
	if err := app.Commit(); err != nil {
		return nil, err
	}

	if err := app.Start(ctx); err != nil {
		return nil, err
	}

	return app, nil
}

func waitForHeight(ctx context.Context, executor, app *instance.Instance, height int64) error {
	for {
		select {
		case <-ctx.Done():
			if ctx.Err() != nil {
				return fmt.Errorf("operation canceled: %v", ctx.Err())
			}
			return nil
		case <-time.After(1 * time.Second):
			status, err := getStatus(ctx, executor, app)
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

func getStatus(ctx context.Context, executor, app *instance.Instance) (string, error) {
	appIP, err := app.GetIP(ctx)
	if err != nil {
		return "", err
	}

	return executor.ExecuteCommand(ctx, "wget", "-q", "-O", "-", fmt.Sprintf("%s:%d/status", appIP, appPortRPC))
}

func latestBlockHeightFromStatus(status string) (int64, error) {
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(status), &result); err != nil {
		return 0, err
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

func createExecutor(ctx context.Context, kn *knuu.Knuu) (*instance.Instance, error) {
	exe, err := kn.NewInstance(executorName)
	if err != nil {
		return nil, err
	}

	if err := exe.SetImage(ctx, executorImage); err != nil {
		return nil, err
	}

	if err := exe.Commit(); err != nil {
		return nil, err
	}

	if err := exe.SetArgs(sleepCommand, infinityArg); err != nil {
		return nil, err
	}

	if err := exe.SetMemory(executorMemoryLimit, executorMemoryLimit); err != nil {
		return nil, err
	}

	if err := exe.SetCPU(executorCpuLimit); err != nil {
		return nil, err
	}

	if err := exe.Start(ctx); err != nil {
		return nil, err
	}

	return exe, nil
}

func chainId(ctx context.Context, executor, app *instance.Instance) (string, error) {
	status, err := getStatus(ctx, executor, app)
	if err != nil {
		return "", fmt.Errorf("error getting status: %v", err)
	}
	chainId, err := chainIdFromStatus(status)
	if err != nil {
		return "", fmt.Errorf("error getting chain id: %w", err)
	}
	return chainId, nil
}

func chainIdFromStatus(status string) (string, error) {
	var result map[string]interface{}
	err := json.Unmarshal([]byte(status), &result)
	if err != nil {
		return "", fmt.Errorf("error unmarshaling status: %w", err)
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
		return "", fmt.Errorf("error unmarshaling block: %w", err)
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

func genesisHash(ctx context.Context, executor, app *instance.Instance) (string, error) {
	appIP, err := app.GetIP(ctx)
	if err != nil {
		return "", fmt.Errorf("error getting app ip: %w", err)
	}
	block, err := executor.ExecuteCommand(ctx, "wget", "-q", "-O", "-", fmt.Sprintf("%s:%d/block?height=1", appIP, appPortRPC))
	if err != nil {
		return "", fmt.Errorf("error getting block: %v", err)
	}
	genesisHash, err := hashFromBlock(block)
	if err != nil {
		return "", fmt.Errorf("error getting hash from block: %v", err)
	}
	return genesisHash, nil
}

func initDaNodeInstance(ctx context.Context, kn *knuu.Knuu, instanceName, nodeType, chainId, genesisHash string) (*instance.Instance, error) {
	instance, err := kn.NewInstance(instanceName)
	if err != nil {
		return nil, err
	}

	if err := instance.SetImage(ctx, nodeImage); err != nil {
		return nil, err
	}

	if err := instance.AddPortTCP(nodePort); err != nil {
		return nil, err
	}
	if err := instance.AddPortTCP(nodePortGRPC); err != nil {
		return nil, err
	}

	if err := instance.SetEnvironmentVariable(nodeEnvCustom, fmt.Sprintf("%s:%s", chainId, genesisHash)); err != nil {
		return nil, err
	}

	if _, err := instance.ExecuteCommand(ctx, celestiaCommand, nodeType, "init", nodeStoreFlag, nodeHomeDir); err != nil {
		return nil, err
	}

	if err := instance.Commit(); err != nil {
		return nil, err
	}
	return instance, nil
}

func createBridge(ctx context.Context, kn *knuu.Knuu, instanceName string, executor, app *instance.Instance) (*instance.Instance, error) {
	chainId, err := chainId(ctx, executor, app)
	if err != nil {
		return nil, err
	}

	genesisHash, err := genesisHash(ctx, executor, app)
	if err != nil {
		return nil, err
	}

	appIP, err := app.GetIP(ctx)
	if err != nil {
		return nil, err
	}

	bridge, err := initDaNodeInstance(ctx, kn, instanceName, nodeTypeBridge, chainId, genesisHash)
	if err != nil {
		return nil, err
	}

	err = bridge.SetCommand(
		celestiaCommand,
		nodeTypeBridge,
		"start",
		nodeStoreFlag, nodeHomeDir,
		"--core.ip", appIP,
		"--metrics",
		"--metrics.tls=false",
		"--p2p.metrics",
		"--tracing",
		"--tracing.tls=false",
	)
	if err != nil {
		return nil, err
	}

	return bridge, nil
}

func createNode(ctx context.Context, kn *knuu.Knuu,
	instanceName string, nodeType string,
	executor, app, trustedNode *instance.Instance,
) (*instance.Instance, error) {
	chainId, err := chainId(ctx, executor, app)
	if err != nil {
		return nil, err
	}
	genesisHash, err := genesisHash(ctx, executor, app)
	if err != nil {
		return nil, err
	}

	node, err := initDaNodeInstance(ctx, kn, instanceName, nodeType, chainId, genesisHash)
	if err != nil {
		return nil, err
	}

	// adminAuthNode, err := trustedNode.ExecuteCommand(ctx, celestiaCommand, "full", "auth", "admin", nodeStoreFlag, nodeHomeDir)
	// if err != nil {
	// 	return nil, err
	// }
	// adminAuthNodeToken, err := authTokenFromAuth(adminAuthNode)
	// if err != nil {
	// 	return nil, err
	// }

	p2pInfoNode, err := trustedNode.ExecuteCommand(ctx, celestiaCommand, "p2p", "info", nodeStoreFlag, nodeHomeDir)
	if err != nil {
		return nil, err
	}

	bridgeIP, err := trustedNode.GetIP(ctx)
	if err != nil {
		return nil, err
	}
	bridgeID, err := iDFromP2PInfo(p2pInfoNode)
	if err != nil {
		return nil, err
	}
	trustedPeers := fmt.Sprintf("/ip4/%s/tcp/2121/p2p/%s", bridgeIP, bridgeID)

	err = node.SetCommand(
		celestiaCommand,
		nodeType, "start",
		nodeStoreFlag, nodeHomeDir,
		"--headers.trusted-peers", trustedPeers,
		"--metrics",
		"--metrics.tls=false",
		"--p2p.metrics",
		"--tracing",
		"--tracing.tls=false",
		"--rpc.addr", "0.0.0.0",
	)
	if err != nil {
		return nil, err
	}

	return node, nil
}

func authTokenFromAuth(auth string) string {
	// Use regex to match the JWT token
	re := regexp.MustCompile(`[A-Za-z0-9\-_=]+\.[A-Za-z0-9\-_=]+\.?[A-Za-z0-9\-_=]*`)
	return re.FindString(auth)
}

func iDFromP2PInfo(p2pInfo string) (string, error) {
	var result map[string]interface{}
	err := json.Unmarshal([]byte(p2pInfo), &result)
	if err != nil {
		return "", fmt.Errorf("error unmarshaling p2p info: %w", err)
	}
	resultData := result["result"].(map[string]interface{})
	id := resultData["id"].(string)
	return id, nil
}

type Client struct {
	Das das.API
}

func getDaNodeRPCClient(ctx context.Context, node *instance.Instance) (*client.Client, error) {
	nodeIP, err := node.GetIP(ctx)
	if err != nil {
		return nil, err
	}

	hostURL, err := node.AddHostWithReadyCheck(ctx, nodePortGRPC, func(host string) (bool, error) {
		// url := strings.Replace(host, "http://", "ws://", 1)
		url := fmt.Sprintf("ws://%s:%d", nodeIP, nodePortGRPC)
		fmt.Printf("url: %v\n", url)
		var client Client
		constructedClient, err := clientbuilder.NewClient(ctx, url, "", &client)
		if err != nil {

			fmt.Printf("clientbuilder.NewClient err: %v\n", err)
			return false, nil
			// return false, err
		}

		client = constructedClient.(Client)
		stats, err := client.Das.SamplingStats(ctx)
		if err != nil {
			fmt.Printf("client.Das.SamplingStats err: %v\n", err)
			return false, nil
			// return false, err
		}

		fmt.Printf("stats: %v\n", stats)

		// fmt.Printf("Checking if host is ready: %s\n", host)
		// out, err := http.Get(host)
		// if err != nil {
		// 	return false, err
		// }
		// defer out.Body.Close()
		// outtxt, err := io.ReadAll(out.Body)
		// fmt.Printf("io.ReadAll err: %v\n", err)
		// fmt.Printf("out txt: %s\n", outtxt)
		return true, nil
		// return err == nil, err
	})
	if err != nil {
		return nil, fmt.Errorf("error adding host to the proxy: %w", err)
	}

	adminAuthNode, err := node.ExecuteCommand(ctx, celestiaCommand, nodeTypeBridge, "auth", "admin", nodeStoreFlag, nodeHomeDir)
	if err != nil {
		return nil, err
	}
	adminAuthNodeToken := authTokenFromAuth(adminAuthNode)

	fmt.Printf("hostURL: %v\n", hostURL)
	fmt.Printf("adminAuthNodeToken: %q\n", adminAuthNodeToken)

	return client.NewClient(ctx, hostURL, adminAuthNodeToken)
}
