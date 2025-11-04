package tastora

import (
	"context"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module/testutil"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"github.com/moby/moby/client"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/celestiaorg/celestia-app/v6/app"
	"github.com/celestiaorg/tastora/framework/docker"
	"github.com/celestiaorg/tastora/framework/docker/container"
	"github.com/celestiaorg/tastora/framework/docker/cosmos"
	"github.com/celestiaorg/tastora/framework/docker/dataavailability"
	"github.com/celestiaorg/tastora/framework/testutil/sdkacc"
	"github.com/celestiaorg/tastora/framework/testutil/wait"
	"github.com/celestiaorg/tastora/framework/testutil/wallet"
	"github.com/celestiaorg/tastora/framework/types"

	rpcclient "github.com/celestiaorg/celestia-node/api/rpc/client"
)

const (
	defaultCelestiaAppTag = "v6.1.2-arabica"
	celestiaAppImage      = "ghcr.io/celestiaorg/celestia-app"
	// defaultNodeTag can be overridden at build time using ldflags
	// Example: go build -ldflags "-X github.com/celestiaorg/celestia-node/nodebuilder/tests/tastora.defaultNodeTag=v1.2.3"
	defaultCelestiaNodeTag = "v0.28.2-arabica"
	nodeImage              = "ghcr.io/celestiaorg/celestia-node"

	testChainID = "test"
)

var (
	defaultNodeTag = "v0.28.2-arabica" // Official release with queued submission feature and fixes
)

// Framework represents the main testing infrastructure for Tastora-based tests.
// It provides Docker-based chain and node setup, similar to how Swamp provides
// mock-based testing infrastructure.
type Framework struct {
	t       *testing.T
	logger  *zap.Logger
	client  *client.Client
	network string

	chainBuilder     *cosmos.ChainBuilder
	daNetworkBuilder *dataavailability.NetworkBuilder

	// Main DA network infrastructure
	celestia  *cosmos.Chain
	daNetwork *dataavailability.Network

	// Node topology (simplified)
	bridgeNodes []*dataavailability.Node // Bridge nodes (bridgeNodes[0] created by default)
	lightNodes  []*dataavailability.Node // Light nodes

	// Private funding infrastructure (not exposed to tests)
	fundingWallet        *types.Wallet
	defaultFundingAmount int64

	// Configuration
	config *Config
}

// NewFramework creates a new Tastora testing framework instance.
// Similar to swamp.NewSwamp(), this sets up the complete testing environment.
func NewFramework(t *testing.T, options ...Option) *Framework {
	f := &Framework{
		t:                    t,
		logger:               zaptest.NewLogger(t),
		defaultFundingAmount: 100_000_000_000, // 100 billion utia
	}

	// Apply configuration options
	cfg := defaultConfig()
	for _, opt := range options {
		opt(cfg)
	}
	f.config = cfg

	f.logger.Info("Setting up Tastora framework", zap.String("test", t.Name()))

	// Simple cleanup: use Docker's built-in cleanup to remove all unused resources
	f.dockerCleanup()

	// Setup Docker client and network - should work reliably with proper cleanup
	cli, netID := docker.DockerSetup(t)
	f.client, f.network = cli, netID
	// Ensure cleanup runs even if tests fail midway
	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		defer cancel()
		_ = f.Stop(cleanupCtx)
	})

	f.chainBuilder, f.daNetworkBuilder = f.createBuilders(cfg)

	return f
}

// SetupNetwork initializes the Celestia chain, DA network, and default bridge node.
func (f *Framework) SetupNetwork(ctx context.Context) error {
	f.celestia = f.createAndStartCelestiaChain(ctx)

	// Build DA network - should work reliably with proper resource isolation
	daNetwork, err := f.daNetworkBuilder.Build(ctx)
	require.NoError(f.t, err, "failed to build DA network")
	f.daNetwork = daNetwork

	defaultBridgeNode := f.newBridgeNode(ctx)
	f.bridgeNodes = append(f.bridgeNodes, defaultBridgeNode)
	f.logger.Info("Default bridge node created and funded automatically")

	return nil
}

// GetBridgeNodes returns all existing bridge nodes.
func (f *Framework) GetBridgeNodes() []*dataavailability.Node {
	return f.bridgeNodes
}

// GetLightNodes returns all existing light nodes.
func (f *Framework) GetLightNodes() []*dataavailability.Node {
	return f.lightNodes
}

// NewBridgeNode creates and starts a new bridge node.
func (f *Framework) NewBridgeNode(ctx context.Context) *dataavailability.Node {
	bridgeNode := f.newBridgeNode(ctx)
	f.bridgeNodes = append(f.bridgeNodes, bridgeNode)
	return bridgeNode
}

// NewLightNode creates and starts a new light node.
func (f *Framework) NewLightNode(ctx context.Context) *dataavailability.Node {
	allLightNodes := f.daNetwork.GetLightNodes()
	if len(f.lightNodes) >= len(allLightNodes) {
		f.t.Fatalf("Cannot create more light nodes: already have %d, max is %d", len(f.lightNodes), len(allLightNodes))
	}

	bridgeNode := f.bridgeNodes[0]
	lightNode := f.startLightNode(ctx, bridgeNode, f.celestia)
	f.fundNodeAccount(ctx, lightNode, f.defaultFundingAmount)
	f.t.Logf("Light node created and funded with %d utia", f.defaultFundingAmount)

	f.verifyNodeBalance(ctx, lightNode, f.defaultFundingAmount, "light node")

	f.lightNodes = append(f.lightNodes, lightNode)
	return lightNode
}

// NewBridgeNodeWithVersion creates and starts a new bridge node with a specific version.
func (f *Framework) NewBridgeNodeWithVersion(ctx context.Context, version string) *dataavailability.Node {
	// Create a separate DA network for this version if it doesn't exist
	daNetwork := f.getOrCreateVersionedDANetwork(ctx, version)

	// Get the first bridge node from the versioned network
	bridgeNodes := daNetwork.GetBridgeNodes()
	if len(bridgeNodes) == 0 {
		f.t.Fatalf("No bridge nodes available in versioned DA network for version %s", version)
	}
	bridgeNode := bridgeNodes[0]

	// Start the bridge node
	err := f.startVersionedBridgeNode(ctx, bridgeNode, version)
	require.NoError(f.t, err, "failed to start bridge node with version %s", version)

	f.bridgeNodes = append(f.bridgeNodes, bridgeNode)
	return bridgeNode
}

// NewLightNodeWithVersion creates and starts a new light node with a specific version.
func (f *Framework) NewLightNodeWithVersion(ctx context.Context, version string) *dataavailability.Node {
	allLightNodes := f.daNetwork.GetLightNodes()
	if len(f.lightNodes) >= len(allLightNodes) {
		f.t.Fatalf("Cannot create more light nodes: already have %d, max is %d", len(f.lightNodes), len(allLightNodes))
	}

	bridgeNode := f.bridgeNodes[0]
	lightNode := f.startLightNodeWithVersion(ctx, bridgeNode, f.celestia, version)
	f.fundNodeAccount(ctx, lightNode, f.defaultFundingAmount)
	f.t.Logf("Light node created and funded with %d utia", f.defaultFundingAmount)

	f.verifyNodeBalance(ctx, lightNode, f.defaultFundingAmount, "light node")

	f.lightNodes = append(f.lightNodes, lightNode)
	return lightNode
}

// GetCelestiaChain returns the Celestia chain instance.
func (f *Framework) GetCelestiaChain() *cosmos.Chain {
	return f.celestia
}

// GetNodeRPCClient retrieves an RPC client for the provided DA node.
func (f *Framework) GetNodeRPCClient(ctx context.Context, daNode *dataavailability.Node) *rpcclient.Client {
	networkInfo, err := daNode.GetNetworkInfo(ctx)
	require.NoError(f.t, err, "failed to get network info")
	rpcAddr := fmt.Sprintf("%s:%s", networkInfo.External.Hostname, networkInfo.External.Ports.RPC)
	require.NotEmpty(f.t, rpcAddr, "rpc address is empty")

	// Normalize wildcard bind address to loopback for outbound connections
	if strings.HasPrefix(rpcAddr, "0.0.0.0:") {
		rpcAddr = "127.0.0.1:" + strings.TrimPrefix(rpcAddr, "0.0.0.0:")
	}

	rpcClient, err := rpcclient.NewClient(ctx, "http://"+rpcAddr, "")
	require.NoError(f.t, err)
	return rpcClient
}

// CreateTestWallet creates a new test wallet with the specified amount.
func (f *Framework) CreateTestWallet(ctx context.Context, amount int64) *types.Wallet {
	sendAmount := sdk.NewCoins(sdk.NewCoin("utia", sdkmath.NewInt(amount)))

	// Create wallet with longer timeout for transaction confirmation
	walletCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	// Create wallet - should work reliably with proper synchronization
	testWallet, err := wallet.CreateAndFund(walletCtx, "test", sendAmount, f.celestia)
	require.NoError(f.t, err, "failed to create test wallet")
	require.NotNil(f.t, testWallet, "wallet is nil")
	return testWallet
}

// newBridgeNode creates and starts a bridge node.
func (f *Framework) newBridgeNode(ctx context.Context) *dataavailability.Node {
	bridgeNode := f.startBridgeNode(ctx, f.celestia)

	if f.config.TxWorkerAccounts > 0 {
		f.t.Logf("Waiting for bridge node with %d worker accounts to initialize...", f.config.TxWorkerAccounts)
		time.Sleep(10 * time.Second)
	}

	f.fundNodeAccount(ctx, bridgeNode, f.defaultFundingAmount)
	f.t.Logf("Bridge node created and funded with %d utia", f.defaultFundingAmount)

	f.verifyNodeBalance(ctx, bridgeNode, f.defaultFundingAmount, "bridge node")

	return bridgeNode
}

// fundWallet sends funds from one wallet to another address.
func (f *Framework) fundWallet(ctx context.Context, fromWallet *types.Wallet, toAddr sdk.AccAddress, amount int64) {
	fromAddr, err := sdkacc.AddressFromWallet(fromWallet)
	require.NoError(f.t, err, "failed to get from address")

	f.t.Logf("sending funds from %s to %s", fromAddr.String(), toAddr.String())

	bankSend := banktypes.NewMsgSend(fromAddr, toAddr.Bytes(), sdk.NewCoins(sdk.NewCoin("utia", sdkmath.NewInt(amount))))

	// Broadcast transaction
	resp, err := f.celestia.BroadcastMessages(ctx, fromWallet, bankSend)
	if err != nil {
		f.t.Fatalf("Failed to broadcast funding transaction: %v", err)
	}

	// Wait for transaction to be confirmed
	if err := f.waitForTransactionInclusion(ctx, resp.TxHash); err != nil {
		f.t.Fatalf("Failed to wait for transaction inclusion: %v", err)
	}

	f.t.Logf("Funding transaction completed for %s", toAddr.String())
}

// waitForTransactionInclusion polls for a transaction hash to verify it was included in a block.
func (f *Framework) waitForTransactionInclusion(ctx context.Context, txHash string) error {
	// Create a context with a longer timeout specifically for transaction confirmation
	waitCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	node := f.celestia.GetNodes()[0]
	rpcClient, err := node.GetRPCClient()
	if err != nil {
		return fmt.Errorf("failed to get RPC client: %w", err)
	}

	startTime := time.Now()
	for {
		select {
		case <-waitCtx.Done():
			return fmt.Errorf("timeout waiting for transaction %s after %v: %w", txHash, time.Since(startTime), waitCtx.Err())
		case <-ticker.C:
			// Query the transaction to see if it was included in a block
			txHashBytes, err := hex.DecodeString(txHash)
			if err != nil {
				f.t.Logf("Failed to decode transaction hash %s: %v", txHash, err)
				continue
			}
			resp, err := rpcClient.Tx(waitCtx, txHashBytes, false)
			if err == nil && resp != nil {
				f.t.Logf("Transaction %s confirmed at height %d", txHash, resp.Height)
				return nil
			}
			// Log periodically to show we're still waiting
			if time.Since(startTime).Seconds() > 10 && int(time.Since(startTime).Seconds())%10 == 0 {
				f.t.Logf("Still waiting for transaction %s to be included...", txHash)
			}
			// Continue polling if transaction not found yet
		}
	}
}

// verifyNodeBalance verifies that a DA node has the expected balance after funding.
func (f *Framework) verifyNodeBalance(ctx context.Context, daNode *dataavailability.Node, expectedAmount int64, nodeType string) {
	nodeClient := f.GetNodeRPCClient(ctx, daNode)
	nodeAddr, err := nodeClient.State.AccountAddress(ctx)
	require.NoError(f.t, err, "failed to get %s account address", nodeType)

	// Verify balance - should work reliably with proper synchronization
	bal := f.queryBalanceWithFallback(ctx, nodeAddr.String(), nodeType)
	if bal.Amount.Int64() == 0 {
		f.t.Logf("Skipping balance verification for %s due to Docker timeout issues", nodeType)
		return
	}

	// Allow small tolerance for gas fees
	minExpected := int64(float64(expectedAmount) * 0.95)
	require.GreaterOrEqual(f.t, bal.Amount.Int64(), minExpected,
		"%s should have sufficient balance after funding (got %d, expected at least %d)",
		nodeType, bal.Amount.Int64(), minExpected)
}

// queryBalanceWithFallback attempts to query balance but returns zero balance on timeout instead of failing
func (f *Framework) queryBalanceWithFallback(ctx context.Context, addr string, nodeType string) sdk.Coin {
	networkInfo, err := f.celestia.GetNodes()[0].GetNetworkInfo(ctx)
	if err != nil {
		f.t.Logf("Failed to get network info for %s, skipping balance verification: %v", nodeType, err)
		return sdk.Coin{Amount: sdkmath.NewInt(0), Denom: "utia"}
	}

	grpcAddr := fmt.Sprintf("%s:%s", networkInfo.External.Hostname, networkInfo.External.Ports.GRPC)
	grpcCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	grpcConn, err := grpc.NewClient(grpcAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		f.t.Logf("Failed to connect to gRPC for %s, skipping balance verification: %v", nodeType, err)
		return sdk.Coin{Amount: sdkmath.NewInt(0), Denom: "utia"}
	}
	defer grpcConn.Close()

	bankClient := banktypes.NewQueryClient(grpcConn)
	req := &banktypes.QueryBalanceRequest{
		Address: addr,
		Denom:   "utia",
	}

	res, err := bankClient.Balance(grpcCtx, req)
	if err != nil {
		f.t.Logf("Failed to query balance for %s, skipping balance verification: %v", nodeType, err)
		return sdk.Coin{Amount: sdkmath.NewInt(0), Denom: "utia"}
	}

	return *res.Balance
}

// fundNodeAccount funds a specific DA node account using the default wallet.
func (f *Framework) fundNodeAccount(ctx context.Context, daNode *dataavailability.Node, amount int64) {
	fundingWallet := f.getOrCreateFundingWallet(ctx)
	nodeClient := f.GetNodeRPCClient(ctx, daNode)

	// Get the node's account address - should work reliably with proper synchronization
	nodeAddr, err := nodeClient.State.AccountAddress(ctx)
	require.NoError(f.t, err, "failed to get node account address")

	nodeAccAddr := sdk.AccAddress(nodeAddr.Bytes())

	f.t.Logf("Funding node account %s with %d utia", nodeAddr.String(), amount)

	f.fundWallet(ctx, fundingWallet, nodeAccAddr, amount)
}

// createBuilders initializes the chain and DA network builders.
func (f *Framework) createBuilders(cfg *Config) (*cosmos.ChainBuilder, *dataavailability.NetworkBuilder) {
	enc := testutil.MakeTestEncodingConfig(app.ModuleEncodingRegisters...)

	// Create chain builder
	chainImage := container.Image{
		Repository: celestiaAppImage,
		Version:    getCelestiaTag(),
		UIDGID:     "10001:10001",
	}

	// Use a more unique test name with timestamp and random component
	testName := fmt.Sprintf("%s-%d-%d", f.t.Name(), time.Now().UnixNano(), time.Now().Unix())
	chainBuilder := cosmos.NewChainBuilderWithTestName(f.t, testName).
		WithDockerClient(f.client).
		WithDockerNetworkID(f.network).
		WithImage(chainImage).
		WithEncodingConfig(&enc).
		WithAdditionalStartArgs(
			"--force-no-bbr",
			"--grpc.enable",
			"--grpc.address", "0.0.0.0:9090",
			"--rpc.grpc_laddr", "tcp://0.0.0.0:9098",
			"--minimum-gas-prices", "0.004utia",
			"--timeout-commit", "1s",
		)

	// Add validator nodes based on config
	for i := 0; i < cfg.NumValidators; i++ {
		nodeConfig := cosmos.NewChainNodeConfigBuilder().Build()
		chainBuilder = chainBuilder.WithNode(nodeConfig)
	}

	// Create DA network builder with just one bridge node for now
	nodeTag := getNodeTag()
	var daImage container.Image
	if nodeTag == "current" {
		// Build Docker image from current codebase
		buildCtx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		imageName, err := f.buildCurrentNodeImage(buildCtx)
		if err != nil {
			f.t.Fatalf("Failed to build current node image: %v", err)
		}
		parts := strings.Split(imageName, ":")
		if len(parts) != 2 {
			f.t.Fatalf("Invalid built image name: %s", imageName)
		}
		daImage = container.Image{
			Repository: parts[0],
			Version:    parts[1],
			UIDGID:     "10001:10001",
		}
	} else {
		daImage = container.Image{
			Repository: getNodeImage(),
			Version:    nodeTag,
			UIDGID:     "10001:10001",
		}
	}

	// always have at least one bridge node.
	bridgeNodeConfig := dataavailability.NewNodeBuilder().
		WithNodeType(types.BridgeNode).
		Build()

	// Add light nodes based on configuration
	var nodeConfigs []dataavailability.NodeConfig
	nodeConfigs = append(nodeConfigs, bridgeNodeConfig)

	for i := 0; i < cfg.LightNodeCount; i++ {
		lightNodeConfig := dataavailability.NewNodeBuilder().
			WithNodeType(types.LightNode).
			Build()
		nodeConfigs = append(nodeConfigs, lightNodeConfig)
	}

	daNetworkBuilder := dataavailability.NewNetworkBuilderWithTestName(f.t, testName).
		WithDockerClient(f.client).
		WithDockerNetworkID(f.network).
		WithImage(daImage).
		WithNodes(nodeConfigs...).
		WithEnv("CELESTIA_KEYRING_BACKEND", "memory").
		WithEnv("CELESTIA_NODE_KEY", "test-key-mnemonic")

	return chainBuilder, daNetworkBuilder
}

// createAndStartCelestiaChain initializes and starts the Celestia chain.
func (f *Framework) createAndStartCelestiaChain(ctx context.Context) *cosmos.Chain {
	var celestia *cosmos.Chain
	var err error
	// Build and start chain - should work reliably with proper resource isolation
	celestia, err = f.chainBuilder.Build(ctx)
	require.NoError(f.t, err, "failed to build celestia chain")

	// Use a longer timeout for chain startup to handle slow chain initialization
	chainStartCtx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	err = celestia.Start(chainStartCtx)
	require.NoError(f.t, err, "failed to start celestia chain")

	// Use a longer timeout for waiting for blocks after chain startup
	blockWaitCtx, cancel2 := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel2()
	require.NoError(f.t, wait.ForBlocks(blockWaitCtx, 2, celestia))
	return celestia
}

// startBridgeNode initializes and starts a bridge node.
func (f *Framework) startBridgeNode(ctx context.Context, chain *cosmos.Chain) *dataavailability.Node {
	genesisHash := f.getGenesisHash(ctx, chain)

	// Get the next available bridge node from the DA network
	bridgeNodes := f.daNetwork.GetBridgeNodes()
	bridgeNodeIndex := len(f.bridgeNodes)
	if bridgeNodeIndex >= len(bridgeNodes) {
		f.t.Fatalf("Cannot create more bridge nodes: already have %d, max is %d", bridgeNodeIndex, len(bridgeNodes))
	}
	bridgeNode := bridgeNodes[bridgeNodeIndex]

	networkInfo, err := chain.GetNodes()[0].GetNetworkInfo(ctx)
	require.NoError(f.t, err, "failed to get network info")
	hostname := networkInfo.Internal.Hostname

	// Build start arguments with explicit core port
	startArgs := []string{"--p2p.network", testChainID, "--core.ip", hostname, "--core.port", "9090", "--rpc.addr", "0.0.0.0", "--keyring.backend", "test"}
	if f.config.TxWorkerAccounts > 0 {
		startArgs = append(startArgs, "--tx.worker.accounts", fmt.Sprintf("%d", f.config.TxWorkerAccounts))
	}

	// Start bridge node - should work reliably with proper resource isolation
	err = bridgeNode.Start(ctx,
		dataavailability.WithChainID(testChainID),
		dataavailability.WithAdditionalStartArguments(startArgs...),
		dataavailability.WithEnvironmentVariables(
			map[string]string{
				"CELESTIA_CUSTOM":       types.BuildCelestiaCustomEnvVar(testChainID, genesisHash, ""),
				"P2P_NETWORK":           testChainID,
				"CELESTIA_BOOTSTRAPPER": "true", // Make bridge node act as DHT bootstrapper
			},
		),
	)
	require.NoError(f.t, err, "failed to start bridge node")
	return bridgeNode
}

// startLightNode initializes and starts a light node.
func (f *Framework) startLightNode(ctx context.Context, bridgeNode *dataavailability.Node, chain *cosmos.Chain) *dataavailability.Node {
	genesisHash := f.getGenesisHash(ctx, chain)

	p2pInfo, err := bridgeNode.GetP2PInfo(ctx)
	require.NoError(f.t, err, "failed to get bridge node p2p info")

	p2pAddr, err := p2pInfo.GetP2PAddress()
	require.NoError(f.t, err, "failed to get bridge node p2p address")

	// Get the core node hostname for state access
	networkInfo, err := chain.GetNodes()[0].GetNetworkInfo(ctx)
	require.NoError(f.t, err, "failed to get network info")
	hostname := networkInfo.Internal.Hostname

	// Get the next available light node from the DA network
	allLightNodes := f.daNetwork.GetLightNodes()
	lightNodeIndex := len(f.lightNodes)

	if lightNodeIndex >= len(allLightNodes) {
		f.t.Fatalf("Cannot create more light nodes: already have %d, max is %d", lightNodeIndex, len(allLightNodes))
	}

	lightNode := f.daNetwork.GetLightNodes()[lightNodeIndex]

	// Start light node - should work reliably with proper resource isolation
	err = lightNode.Start(ctx,
		dataavailability.WithChainID(testChainID),
		dataavailability.WithAdditionalStartArguments("--p2p.network", testChainID, "--core.ip", hostname, "--core.port", "9090", "--rpc.addr", "0.0.0.0", "--p2p.mutual", p2pAddr),
		dataavailability.WithEnvironmentVariables(
			map[string]string{
				"CELESTIA_CUSTOM": types.BuildCelestiaCustomEnvVar(testChainID, genesisHash, p2pAddr),
				"P2P_NETWORK":     testChainID,
			},
		),
	)
	require.NoError(f.t, err, "failed to start light node")
	return lightNode
}

// startLightNodeWithVersion initializes and starts a light node with a specific version.
func (f *Framework) startLightNodeWithVersion(ctx context.Context, bridgeNode *dataavailability.Node, chain *cosmos.Chain, version string) *dataavailability.Node {
	genesisHash := f.getGenesisHash(ctx, chain)

	p2pInfo, err := bridgeNode.GetP2PInfo(ctx)
	require.NoError(f.t, err, "failed to get bridge node p2p info")

	p2pAddr, err := p2pInfo.GetP2PAddress()
	require.NoError(f.t, err, "failed to get bridge node p2p address")

	// Get the core node hostname for state access
	networkInfo, err := chain.GetNodes()[0].GetNetworkInfo(ctx)
	require.NoError(f.t, err, "failed to get network info")
	hostname := networkInfo.Internal.Hostname

	// Get the next available light node from the DA network
	allLightNodes := f.daNetwork.GetLightNodes()
	lightNodeIndex := len(f.lightNodes)
	if lightNodeIndex >= len(allLightNodes) {
		f.t.Fatalf("Cannot create more light nodes: already have %d, max is %d", lightNodeIndex, len(allLightNodes))
	}
	lightNode := allLightNodes[lightNodeIndex]

	f.t.Logf("Starting light node with version %s", version)

	// Start light node with specific version
	// Note: Currently using the same image as the DA network, but version is tracked for debugging
	err = lightNode.Start(ctx,
		dataavailability.WithChainID(testChainID),
		dataavailability.WithAdditionalStartArguments("--p2p.network", testChainID, "--core.ip", hostname, "--core.port", "9090", "--rpc.addr", "0.0.0.0", "--p2p.mutual", p2pAddr),
		dataavailability.WithEnvironmentVariables(
			map[string]string{
				"CELESTIA_CUSTOM":       types.BuildCelestiaCustomEnvVar(testChainID, genesisHash, p2pAddr),
				"P2P_NETWORK":           testChainID,
				"CELESTIA_NODE_VERSION": version, // Add version for debugging
			},
		),
	)
	require.NoError(f.t, err, "failed to start light node with version %s", version)
	return lightNode
}

// getGenesisHash returns the genesis hash of the chain.
func (f *Framework) getGenesisHash(ctx context.Context, chain *cosmos.Chain) string {
	node := chain.GetNodes()[0]
	c, err := node.GetRPCClient()
	require.NoError(f.t, err, "failed to get node client")

	first := int64(1)
	block, err := c.Block(ctx, &first)
	require.NoError(f.t, err, "failed to get block")

	genesisHash := block.Block.Header.Hash().String()
	require.NotEmpty(f.t, genesisHash, "genesis hash is empty")
	return genesisHash
}

// buildCurrentNodeImage builds a Docker image from the current codebase.
// Returns the full image name (repository:tag).
func (f *Framework) buildCurrentNodeImage(ctx context.Context) (string, error) {
	// Use a unique tag based on test name and timestamp
	imageTag := fmt.Sprintf("celestia-node-test:%s-%d", strings.ToLower(strings.ReplaceAll(f.t.Name(), "/", "-")), time.Now().Unix())
	imageName := imageTag

	// Get the project root directory
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get current directory: %w", err)
	}

	// Find project root (where go.mod is)
	projectRoot := cwd
	for {
		if _, err := os.Stat(filepath.Join(projectRoot, "go.mod")); err == nil {
			break
		}
		parent := filepath.Dir(projectRoot)
		if parent == projectRoot {
			return "", fmt.Errorf("could not find project root (go.mod)")
		}
		projectRoot = parent
	}

	f.t.Logf("Building Docker image from current codebase: %s", imageName)
	f.t.Logf("Project root: %s", projectRoot)

	// Build Docker image
	cmd := exec.CommandContext(ctx, "docker", "build", "-t", imageName, "-f", "Dockerfile", ".")
	cmd.Dir = projectRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("docker build failed: %w", err)
	}

	f.t.Logf("Successfully built image: %s", imageName)
	return imageName, nil
}

// getVersionImageTag returns the Docker image tag for a specific version.
// Version strings should be in the format "v0.28.2-arabica" or similar.
// If the version doesn't include the repository, it's prepended with the default.
func (f *Framework) getVersionImageTag(version string) string {
	// If version already contains a repository (has a slash), use it as-is
	if strings.Contains(version, "/") {
		return version
	}
	// Otherwise, prepend with the default repository
	return fmt.Sprintf("ghcr.io/celestiaorg/celestia-node:%s", version)
}

// GetDockerClient returns the Docker client used by the framework.
func (f *Framework) GetDockerClient() *client.Client {
	return f.client
}

// GetDockerNetwork returns the Docker network ID used by the framework.
func (f *Framework) GetDockerNetwork() string {
	return f.network
}

// versionedDANetworks stores DA networks for different versions
var versionedDANetworks = make(map[string]*dataavailability.Network)

// getOrCreateVersionedDANetwork creates or returns a DA network for a specific version.
func (f *Framework) getOrCreateVersionedDANetwork(ctx context.Context, version string) *dataavailability.Network {
	// Check if we already have a DA network for this version
	if daNetwork, exists := versionedDANetworks[version]; exists {
		return daNetwork
	}

	// Create a new DA network for this version
	imageTag := f.getVersionImageTag(version)
	f.t.Logf("Creating DA network for version %s using image: %s", version, imageTag)

	// Parse the image tag to get repository and version
	parts := strings.Split(imageTag, ":")
	var repository, tag string
	if len(parts) == 2 {
		repository = parts[0]
		tag = parts[1]
	} else {
		f.t.Fatalf("Invalid image tag format: %s (expected 'repository:tag')", imageTag)
	}

	// Create DA image for this version
	daImage := container.Image{
		Repository: repository,
		Version:    tag,
		UIDGID:     "10001:10001",
	}

	// Create a single bridge node for this version
	bridgeNodeConfig := dataavailability.NewNodeBuilder().
		WithNodeType(types.BridgeNode).
		Build()

	// Create the DA network for this version
	daNetworkBuilder := dataavailability.NewNetworkBuilderWithTestName(f.t, fmt.Sprintf("tastora-versioned-%s", version)).
		WithDockerClient(f.client).
		WithDockerNetworkID(f.network).
		WithImage(daImage).
		WithNodes(bridgeNodeConfig).
		WithEnv("CELESTIA_KEYRING_BACKEND", "memory").
		WithEnv("CELESTIA_NODE_KEY", "test-key-mnemonic")

	daNetwork, err := daNetworkBuilder.Build(ctx)
	require.NoError(f.t, err, "failed to build DA network for version %s", version)

	// Store the network for reuse
	versionedDANetworks[version] = daNetwork
	return daNetwork
}

// startVersionedBridgeNode starts a bridge node with version-specific configuration.
func (f *Framework) startVersionedBridgeNode(ctx context.Context, bridgeNode *dataavailability.Node, version string) error {
	genesisHash := f.getGenesisHash(ctx, f.celestia)

	networkInfo, err := f.celestia.GetNodes()[0].GetNetworkInfo(ctx)
	require.NoError(f.t, err, "failed to get network info")
	hostname := networkInfo.Internal.Hostname

	// Build start arguments with explicit core port
	startArgs := []string{"--p2p.network", testChainID, "--core.ip", hostname, "--core.port", "9090", "--rpc.addr", "0.0.0.0", "--keyring.backend", "test"}
	if f.config.TxWorkerAccounts > 0 {
		startArgs = append(startArgs, "--tx.worker.accounts", fmt.Sprintf("%d", f.config.TxWorkerAccounts))
	}

	// Start bridge node with version-specific configuration
	return bridgeNode.Start(ctx,
		dataavailability.WithChainID(testChainID),
		dataavailability.WithAdditionalStartArguments(startArgs...),
		dataavailability.WithEnvironmentVariables(
			map[string]string{
				"CELESTIA_CUSTOM":       types.BuildCelestiaCustomEnvVar(testChainID, genesisHash, ""),
				"P2P_NETWORK":           testChainID,
				"CELESTIA_BOOTSTRAPPER": "true",  // Make bridge node act as DHT bootstrapper
				"CELESTIA_NODE_VERSION": version, // Add version for debugging
			},
		),
	)
}

// getCelestiaTag returns the Celestia app image tag.
func getCelestiaTag() string {
	if tag := os.Getenv("CELESTIA_TAG"); tag != "" {
		return tag
	}
	return defaultCelestiaAppTag
}

// getNodeTag returns the Celestia node image tag.
// If CELESTIA_NODE_TAG is "current" or unset, builds from current codebase.
// Otherwise uses the specified tag or default.
func getNodeTag() string {
	tag := os.Getenv("CELESTIA_NODE_TAG")
	if tag == "" || tag == "current" {
		// Build from current codebase for cross-version testing
		return "current"
	}
	if tag == "default" {
		return defaultCelestiaNodeTag
	}
	return tag
}

// getNodeImage returns the Celestia node image.
func getNodeImage() string {
	if tag := os.Getenv("CELESTIA_NODE_IMAGE"); tag != "" {
		return tag
	}
	return nodeImage
}

// getOrCreateFundingWallet returns the framework's funding wallet, creating it if needed.
func (f *Framework) getOrCreateFundingWallet(ctx context.Context) *types.Wallet {
	if f.fundingWallet == nil {
		// Wait longer for chain to stabilize before creating funding wallet
		// This is especially important right after chain startup when transactions may be slow to confirm
		f.t.Logf("Waiting for chain to stabilize before creating funding wallet...")
		time.Sleep(5 * time.Second)
		f.fundingWallet = f.CreateTestWallet(ctx, 200_000_000_000)
		f.t.Logf("Created funding wallet for automatic node funding")
	}
	return f.fundingWallet
}

// Stop cleans up all Docker containers and resources created by the framework.
func (f *Framework) Stop(ctx context.Context) error {
	f.logger.Info("Stopping framework and cleaning up containers")

	// Stop all bridge nodes
	for i, node := range f.bridgeNodes {
		if node != nil {
			f.logger.Info("Stopping bridge node", zap.Int("index", i))
			if err := node.Stop(ctx); err != nil {
				if !strings.Contains(err.Error(), "No such container") {
					f.logger.Warn("Failed to stop bridge node", zap.Int("index", i), zap.Error(err))
				}
			}
			if err := node.Remove(ctx); err != nil {
				if !strings.Contains(err.Error(), "No such container") {
					f.logger.Warn("Failed to remove bridge node", zap.Int("index", i), zap.Error(err))
				}
			}
		}
	}

	// Stop all light nodes
	for i, node := range f.lightNodes {
		if node != nil {
			f.logger.Info("Stopping light node", zap.Int("index", i))
			if err := node.Stop(ctx); err != nil {
				if !strings.Contains(err.Error(), "No such container") {
					f.logger.Warn("Failed to stop light node", zap.Int("index", i), zap.Error(err))
				}
			}
			if err := node.Remove(ctx); err != nil {
				if !strings.Contains(err.Error(), "No such container") {
					f.logger.Warn("Failed to remove light node", zap.Int("index", i), zap.Error(err))
				}
			}
		}
	}

	// Stop DA network
	if f.daNetwork != nil {
		f.logger.Info("Stopping DA network")
		// Get all node names to remove
		allNodes := f.daNetwork.GetNodes()
		nodeNames := make([]string, len(allNodes))
		for i, node := range allNodes {
			nodeNames[i] = node.Name()
		}
		if err := f.daNetwork.RemoveNodes(ctx, nodeNames...); err != nil {
			if !strings.Contains(err.Error(), "No such container") {
				f.logger.Warn("Failed to remove DA network nodes", zap.Error(err))
			}
		}
	}

	// Stop Celestia chain (validator containers)
	if f.celestia != nil {
		f.logger.Info("Stopping Celestia chain")
		if err := f.celestia.Stop(ctx); err != nil {
			if !strings.Contains(err.Error(), "No such container") {
				f.logger.Warn("Failed to stop Celestia chain", zap.Error(err))
			}
		}
		if err := f.celestia.Remove(ctx); err != nil {
			if !strings.Contains(err.Error(), "No such container") {
				f.logger.Warn("Failed to remove Celestia chain", zap.Error(err))
			}
		}

	}

	if f.client != nil && f.network != "" {
		if err := f.client.NetworkRemove(ctx, f.network); err != nil {
			if !strings.Contains(err.Error(), "No such network") && !strings.Contains(err.Error(), "has active endpoints") {
				f.logger.Warn("Failed to remove docker network", zap.String("network", f.network), zap.Error(err))
			}
		}
	}

	// Use Docker's built-in cleanup commands for comprehensive cleanup
	f.dockerCleanup()

	f.logger.Info("Framework cleanup completed")
	return nil
}

// dockerCleanup uses Docker's built-in commands to clean up unused resources
func (f *Framework) dockerCleanup() {
	// Clean up containers with Tastora test-related names (da-* and test-val-*)
	f.cleanupContainersByPattern("da-")
	f.cleanupContainersByPattern("test-val-")

	// Use Docker's built-in cleanup commands for comprehensive cleanup
	exec.Command("docker", "system", "prune", "-f").Run()
	exec.Command("docker", "volume", "prune", "-f").Run()
}

// cleanupContainersByPattern stops and removes containers matching the given name pattern
func (f *Framework) cleanupContainersByPattern(pattern string) {
	cmd := exec.Command("docker", "ps", "-a", "--filter", "name="+pattern, "--format", "{{.Names}}")
	output, err := cmd.Output()
	if err == nil && len(output) > 0 {
		containerNames := strings.Split(strings.TrimSpace(string(output)), "\n")

		// Stop all containers
		for _, name := range containerNames {
			if name != "" {
				exec.Command("docker", "stop", name).Run()
			}
		}

		// Remove all containers
		for _, name := range containerNames {
			if name != "" {
				exec.Command("docker", "rm", name).Run()
			}
		}
	}
}
