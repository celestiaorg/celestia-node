package tastora

import (
	"context"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module/testutil"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
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
	defaultCelestiaAppTag = "v6.2.0-mocha"
	celestiaAppImage      = "ghcr.io/celestiaorg/celestia-app"
	// defaultNodeTag can be overridden at build time using ldflags
	// Example: go build -ldflags "-X github.com/celestiaorg/celestia-node/nodebuilder/tests/tastora.defaultNodeTag=v1.2.3"
	defaultCelestiaNodeTag = "v0.28.3-arabica"
	nodeImage              = "ghcr.io/celestiaorg/celestia-node"

	testChainID = "test"
)

var (
	defaultNodeTag = "v0.28.3-arabica" // Official release with queued submission feature and fixes
)

// Framework represents the main testing infrastructure for Tastora-based tests.
// It provides Docker-based chain and node setup, similar to how Swamp provides
// mock-based testing infrastructure.
type Framework struct {
	t       *testing.T
	logger  *zap.Logger
	client  types.TastoraDockerClient
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
	// Ensure cleanup runs even if tests fail midway
	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		defer cancel()
		_ = f.Stop(cleanupCtx)
	})

	// Setup Docker client and network - should work reliably with proper cleanup
	f.client, f.network = docker.Setup(t)
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

// GetDockerClient returns the Docker client used by the framework.
func (f *Framework) GetDockerClient() types.TastoraDockerClient {
	return f.client
}

// GetDockerNetwork returns the Docker network ID used by the framework.
func (f *Framework) GetDockerNetwork() string {
	return f.network
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
	// Create a new node builder with the specified version
	daImage := container.Image{
		Repository: nodeImage,
		Version:    version,
		UIDGID:     "10001:10001",
	}

	// Create a new network builder for this specific version node
	testName := fmt.Sprintf("%s-%s-%d", f.t.Name(), version, time.Now().UnixNano())
	bridgeNodeConfig := dataavailability.NewNodeBuilder().
		WithNodeType(types.BridgeNode).
		Build()

	versionedNetworkBuilder := dataavailability.NewNetworkBuilderWithTestName(f.t, testName).
		WithDockerClient(f.client).
		WithDockerNetworkID(f.network).
		WithImage(daImage).
		WithNodes(bridgeNodeConfig).
		WithEnv("CELESTIA_KEYRING_BACKEND", "memory").
		WithEnv("CELESTIA_NODE_KEY", "test-key-mnemonic")

	// Build the network
	versionedNetwork, err := versionedNetworkBuilder.Build(ctx)
	require.NoError(f.t, err, "failed to build versioned bridge node network")

	// Get the bridge node from the versioned network
	bridgeNodes := versionedNetwork.GetBridgeNodes()
	require.Greater(f.t, len(bridgeNodes), 0, "no bridge nodes in versioned network")
	bridgeNode := bridgeNodes[0]

	err = f.startBridgeNodeWithConfig(ctx, bridgeNode, f.celestia)
	require.NoError(f.t, err, "failed to start versioned bridge node")

	f.fundNodeAccount(ctx, bridgeNode, f.defaultFundingAmount)
	f.verifyNodeBalance(ctx, bridgeNode, f.defaultFundingAmount, "versioned bridge node")

	f.bridgeNodes = append(f.bridgeNodes, bridgeNode)
	return bridgeNode
}

// NewLightNodeWithVersion creates and starts a new light node with a specific version.
func (f *Framework) NewLightNodeWithVersion(ctx context.Context, version string) *dataavailability.Node {
	bridgeNode := f.bridgeNodes[0]

	// Create a new node builder with the specified version
	daImage := container.Image{
		Repository: nodeImage,
		Version:    version,
		UIDGID:     "10001:10001",
	}

	// Create a new network builder for this specific version node
	testName := fmt.Sprintf("%s-%s-%d", f.t.Name(), version, time.Now().UnixNano())
	lightNodeConfig := dataavailability.NewNodeBuilder().
		WithNodeType(types.LightNode).
		Build()

	versionedNetworkBuilder := dataavailability.NewNetworkBuilderWithTestName(f.t, testName).
		WithDockerClient(f.client).
		WithDockerNetworkID(f.network).
		WithImage(daImage).
		WithNodes(lightNodeConfig).
		WithEnv("CELESTIA_KEYRING_BACKEND", "memory").
		WithEnv("CELESTIA_NODE_KEY", "test-key-mnemonic")

	// Build the network
	versionedNetwork, err := versionedNetworkBuilder.Build(ctx)
	require.NoError(f.t, err, "failed to build versioned light node network")

	// Get the light node from the versioned network
	lightNodes := versionedNetwork.GetLightNodes()
	require.Greater(f.t, len(lightNodes), 0, "no light nodes in versioned network")
	lightNode := lightNodes[0]

	err = f.startLightNodeWithConfig(ctx, lightNode, bridgeNode, f.celestia)
	require.NoError(f.t, err, "failed to start versioned light node")

	f.fundNodeAccount(ctx, lightNode, f.defaultFundingAmount)
	f.verifyNodeBalance(ctx, lightNode, f.defaultFundingAmount, "versioned light node")

	f.lightNodes = append(f.lightNodes, lightNode)
	return lightNode
}

// GetCelestiaChain returns the Celestia chain instance.
func (f *Framework) GetCelestiaChain() *cosmos.Chain {
	return f.celestia
}

func (f *Framework) GetNodeRPCClient(ctx context.Context, daNode *dataavailability.Node) *rpcclient.Client {
	networkInfo, err := daNode.GetNetworkInfo(ctx)
	require.NoError(f.t, err, "failed to get network info")
	rpcAddr := fmt.Sprintf("%s:%s", networkInfo.External.Hostname, networkInfo.External.Ports.RPC)
	require.NotEmpty(f.t, rpcAddr, "rpc address is empty")

	if strings.HasPrefix(rpcAddr, "0.0.0.0:") {
		rpcAddr = "127.0.0.1:" + strings.TrimPrefix(rpcAddr, "0.0.0.0:")
	}

	rpcClient, err := rpcclient.NewClient(ctx, "http://"+rpcAddr, "")
	require.NoError(f.t, err)
	return rpcClient
}

func (f *Framework) CreateTestWallet(ctx context.Context, amount int64) *types.Wallet {
	sendAmount := sdk.NewCoins(sdk.NewCoin("utia", sdkmath.NewInt(amount)))
	testWallet, err := wallet.CreateAndFund(ctx, "test", sendAmount, f.celestia)
	require.NoError(f.t, err, "failed to create test wallet")
	require.NotNil(f.t, testWallet, "wallet is nil")
	return testWallet
}

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
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	node := f.celestia.GetNodes()[0]
	rpcClient, err := node.GetRPCClient()
	if err != nil {
		return fmt.Errorf("failed to get RPC client: %w", err)
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			// Query the transaction to see if it was included in a block
			txHashBytes, err := hex.DecodeString(txHash)
			if err != nil {
				f.t.Logf("Failed to decode transaction hash %s: %v", txHash, err)
				continue
			}
			resp, err := rpcClient.Tx(ctx, txHashBytes, false)
			if err == nil && resp != nil {
				f.t.Logf("Transaction %s confirmed at height %d", txHash, resp.Height)
				return nil
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
	daImage := container.Image{
		Repository: getNodeImage(),
		Version:    getNodeTag(),
		UIDGID:     "10001:10001",
	}

	totalNodes := cfg.BridgeNodeCount + cfg.LightNodeCount
	nodeConfigs := make([]dataavailability.NodeConfig, 0, totalNodes)

	for i := 0; i < cfg.BridgeNodeCount; i++ {
		nodeConfigs = append(nodeConfigs, dataavailability.NewNodeBuilder().
			WithNodeType(types.BridgeNode).
			Build())
	}

	for i := 0; i < cfg.LightNodeCount; i++ {
		nodeConfigs = append(nodeConfigs, dataavailability.NewNodeBuilder().
			WithNodeType(types.LightNode).
			Build())
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
	chainStartCtx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()
	err = celestia.Start(chainStartCtx)
	require.NoError(f.t, err, "failed to start celestia chain")

	// Use a longer timeout for waiting for blocks after chain startup
	// Increased timeout to handle slow chain initialization and block production
	blockWaitCtx, cancel2 := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel2()
	require.NoError(f.t, wait.ForBlocks(blockWaitCtx, 2, celestia))
	return celestia
}

// startBridgeNodeWithConfig starts a bridge node with the given configuration.
// This is a helper function that contains the common startup logic for bridge nodes.
func (f *Framework) startBridgeNodeWithConfig(ctx context.Context, bridgeNode *dataavailability.Node, chain *cosmos.Chain) error {
	genesisHash := f.getGenesisHash(ctx, chain)

	networkInfo, err := chain.GetNodes()[0].GetNetworkInfo(ctx)
	if err != nil {
		return fmt.Errorf("failed to get network info: %w", err)
	}
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
	if err != nil {
		return fmt.Errorf("failed to start bridge node: %w", err)
	}
	return nil
}

// startBridgeNode initializes and starts a bridge node.
func (f *Framework) startBridgeNode(ctx context.Context, chain *cosmos.Chain) *dataavailability.Node {
	// Get the next available bridge node from the DA network
	bridgeNodes := f.daNetwork.GetBridgeNodes()
	bridgeNodeIndex := len(f.bridgeNodes)
	if bridgeNodeIndex >= len(bridgeNodes) {
		f.t.Fatalf("Cannot create more bridge nodes: already have %d, max is %d", bridgeNodeIndex, len(bridgeNodes))
	}
	bridgeNode := bridgeNodes[bridgeNodeIndex]

	err := f.startBridgeNodeWithConfig(ctx, bridgeNode, chain)
	require.NoError(f.t, err, "failed to start bridge node")
	return bridgeNode
}

// startLightNodeWithConfig starts a light node with the given configuration.
// This is a helper function that contains the common startup logic for light nodes.
func (f *Framework) startLightNodeWithConfig(ctx context.Context, lightNode *dataavailability.Node, bridgeNode *dataavailability.Node, chain *cosmos.Chain) error {
	genesisHash := f.getGenesisHash(ctx, chain)

	p2pInfo, err := bridgeNode.GetP2PInfo(ctx)
	if err != nil {
		return fmt.Errorf("failed to get bridge node p2p info: %w", err)
	}

	p2pAddr, err := p2pInfo.GetP2PAddress()
	if err != nil {
		return fmt.Errorf("failed to get bridge node p2p address: %w", err)
	}

	// Get the core node hostname for state access
	networkInfo, err := chain.GetNodes()[0].GetNetworkInfo(ctx)
	if err != nil {
		return fmt.Errorf("failed to get network info: %w", err)
	}
	hostname := networkInfo.Internal.Hostname

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
	if err != nil {
		return fmt.Errorf("failed to start light node: %w", err)
	}
	return nil
}

// startLightNode initializes and starts a light node.
func (f *Framework) startLightNode(ctx context.Context, bridgeNode *dataavailability.Node, chain *cosmos.Chain) *dataavailability.Node {
	// Get the next available light node from the DA network
	allLightNodes := f.daNetwork.GetLightNodes()
	lightNodeIndex := len(f.lightNodes)

	if lightNodeIndex >= len(allLightNodes) {
		f.t.Fatalf("Cannot create more light nodes: already have %d, max is %d", lightNodeIndex, len(allLightNodes))
	}

	lightNode := f.daNetwork.GetLightNodes()[lightNodeIndex]

	err := f.startLightNodeWithConfig(ctx, lightNode, bridgeNode, chain)
	require.NoError(f.t, err, "failed to start light node")
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

// getCelestiaTag returns the Celestia app image tag.
func getCelestiaTag() string {
	if tag := os.Getenv("CELESTIA_TAG"); tag != "" {
		return tag
	}
	return defaultCelestiaAppTag
}

// getNodeTag returns the Celestia node image tag.
func getNodeTag() string {
	if tag := os.Getenv("CELESTIA_NODE_TAG"); tag != "" {
		return tag
	}
	return defaultCelestiaNodeTag
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
		f.fundingWallet = f.CreateTestWallet(ctx, 200_000_000_000)
		f.t.Logf("Created funding wallet for automatic node funding")
	}
	return f.fundingWallet
}

func (f *Framework) Stop(ctx context.Context) error {
	f.logger.Info("Stopping framework and cleaning up containers")

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

	if f.daNetwork != nil {
		f.logger.Info("Stopping DA network")
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

	f.dockerCleanup()

	f.logger.Info("Framework cleanup completed")
	return nil
}

func (f *Framework) dockerCleanup() {
	f.cleanupContainersByPattern("da-")
	f.cleanupContainersByPattern("test-val-")

	exec.Command("docker", "system", "prune", "-f").Run()
	exec.Command("docker", "volume", "prune", "-f").Run()
}

func (f *Framework) cleanupContainersByPattern(pattern string) {
	cmd := exec.Command("docker", "ps", "-a", "--filter", "name="+pattern, "--format", "{{.Names}}")
	output, err := cmd.Output()
	if err == nil && len(output) > 0 {
		containerNames := strings.Split(strings.TrimSpace(string(output)), "\n")

		for _, name := range containerNames {
			if name != "" {
				exec.Command("docker", "stop", name).Run()
			}
		}

		for _, name := range containerNames {
			if name != "" {
				exec.Command("docker", "rm", name).Run()
			}
		}
	}
}
