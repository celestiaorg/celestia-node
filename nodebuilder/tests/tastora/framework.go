package tastora

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	sdkmath "cosmossdk.io/math"
	cometcfg "github.com/cometbft/cometbft/config"
	"github.com/containerd/errdefs"
	servercfg "github.com/cosmos/cosmos-sdk/server/config"
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
	"github.com/celestiaorg/celestia-node/state"
	"github.com/celestiaorg/tastora/framework/docker"
	"github.com/celestiaorg/tastora/framework/docker/container"
	"github.com/celestiaorg/tastora/framework/docker/cosmos"
	"github.com/celestiaorg/tastora/framework/docker/dataavailability"
	"github.com/celestiaorg/tastora/framework/testutil/config"
	"github.com/celestiaorg/tastora/framework/testutil/sdkacc"
	"github.com/celestiaorg/tastora/framework/testutil/wait"
	"github.com/celestiaorg/tastora/framework/testutil/wallet"
	"github.com/celestiaorg/tastora/framework/types"

	rpcclient "github.com/celestiaorg/celestia-node/api/rpc/client"
)

const (
	celestiaAppImage   = "ghcr.io/celestiaorg/celestia-app"
	defaultCelestiaTag = "v5.0.1"
	nodeImage          = "ghcr.io/celestiaorg/celestia-node"
	testChainID        = "test"
)

var (
	// defaultNodeTag can be overridden at build time using ldflags
	// Example: go build -ldflags "-X github.com/celestiaorg/celestia-node/nodebuilder/tests/tastora.defaultNodeTag=v1.2.3"
	defaultNodeTag = "v0.28.1-arabica" // Official release with queued submission feature and fixes
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
	f.client, f.network = docker.DockerSetup(t)
	f.chainBuilder, f.daNetworkBuilder = f.createBuilders(cfg)

	return f
}

// SetupNetwork initializes the basic network infrastructure.
// This includes starting the Celestia chain, DA network infrastructure,
// and ALWAYS creates the main bridge node (minimum viable DA network).
func (f *Framework) SetupNetwork(ctx context.Context) error {
	// 1. Create and start Celestia chain
	f.celestia = f.createAndStartCelestiaChain(ctx)

	// 2. Setup DA network infrastructure (retry once on transient docker errors)
	var daNetwork *dataavailability.Network
	var err error
	for attempt := 1; attempt <= 2; attempt++ {
		daNetwork, err = f.daNetworkBuilder.Build(ctx)
		if err == nil {
			break
		}
		if errdefs.IsNotFound(err) {
			f.t.Logf("retrying DA network setup due to missing container (attempt %d/2): %v", attempt, err)
			time.Sleep(2 * time.Second)
			continue
		}
		return err
	}
	f.daNetwork = daNetwork

	// 3. ALWAYS create default bridge node (minimum viable DA network)
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

// NewBridgeNode creates, starts and appends a new bridge node.
func (f *Framework) NewBridgeNode(ctx context.Context) *dataavailability.Node {
	bridgeNode := f.newBridgeNode(ctx)
	f.bridgeNodes = append(f.bridgeNodes, bridgeNode)
	return bridgeNode
}

// NewLightNode creates, starts and appends a new light node.
func (f *Framework) NewLightNode(ctx context.Context) *dataavailability.Node {
	// Get the next available light node from the DA network
	allLightNodes := f.daNetwork.GetLightNodes()
	if len(f.lightNodes) >= len(allLightNodes) {
		f.t.Fatalf("Cannot create more light nodes: already have %d, max is %d", len(f.lightNodes), len(allLightNodes))
	}

	// Use the first bridge node as the connection point
	bridgeNode := f.bridgeNodes[0]
	lightNode := f.startLightNode(ctx, bridgeNode, f.celestia)
	f.fundNodeAccount(ctx, lightNode, f.defaultFundingAmount)
	f.t.Logf("Light node created and funded with %d utia", f.defaultFundingAmount)

	// Verify funds are available by checking balance
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

// CreateTestWallet creates a new test wallet on the chain, funding it with the specified amount.
func (f *Framework) CreateTestWallet(ctx context.Context, amount int64) *types.Wallet {
	sendAmount := sdk.NewCoins(sdk.NewCoin("utia", sdkmath.NewInt(amount)))
	testWallet, err := wallet.CreateAndFund(ctx, "test", sendAmount, f.celestia)
	require.NoError(f.t, err, "failed to create test wallet")
	require.NotNil(f.t, testWallet, "wallet is nil")
	return testWallet
}

// queryBalance fetches the balance of a given address.
func (f *Framework) queryBalance(ctx context.Context, addr string) sdk.Coin {
	networkInfo, err := f.celestia.GetNodes()[0].GetNetworkInfo(ctx)
	require.NoError(f.t, err, "failed to get network info")
	grpcAddr := fmt.Sprintf("%s:%s", networkInfo.External.Hostname, networkInfo.External.Ports.GRPC)

	grpcCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	grpcConn, err := grpc.NewClient(grpcAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(f.t, err, "failed to connect to gRPC")

	defer grpcConn.Close()

	bankClient := banktypes.NewQueryClient(grpcConn)
	req := &banktypes.QueryBalanceRequest{
		Address: addr,
		Denom:   "utia",
	}

	res, err := bankClient.Balance(grpcCtx, req)
	require.NoError(f.t, err, "failed to query balance for address %s", addr)
	return *res.Balance
}

// newBridgeNode creates and starts a bridge node.
func (f *Framework) newBridgeNode(ctx context.Context) *dataavailability.Node {
	bridgeNode := f.startBridgeNode(ctx, f.celestia)

	// Allow time for worker account initialization
	if f.config.TxWorkerAccounts > 0 {
		f.t.Logf("Waiting for bridge node with %d worker accounts to initialize...", f.config.TxWorkerAccounts)
		time.Sleep(10 * time.Second)
	}

	f.fundNodeAccount(ctx, bridgeNode, f.defaultFundingAmount)
	f.t.Logf("Bridge node created and funded with %d utia", f.defaultFundingAmount)

	// Verify funds are available by checking balance
	f.verifyNodeBalance(ctx, bridgeNode, f.defaultFundingAmount, "bridge node")

	return bridgeNode
}

// fundWallet sends funds from one wallet to another address.
func (f *Framework) fundWallet(ctx context.Context, fromWallet *types.Wallet, toAddr sdk.AccAddress, amount int64) {
	fromAddr, err := sdkacc.AddressFromWallet(fromWallet)
	require.NoError(f.t, err, "failed to get from address")

	f.t.Logf("sending funds from %s to %s", fromAddr.String(), toAddr.String())

	bankSend := banktypes.NewMsgSend(fromAddr, toAddr.Bytes(), sdk.NewCoins(sdk.NewCoin("utia", sdkmath.NewInt(amount))))

	// Broadcast transaction with retry logic
	var resp sdk.TxResponse
	for attempt := 1; attempt <= 3; attempt++ {
		resp, err = f.celestia.BroadcastMessages(ctx, fromWallet, bankSend)
		if err == nil && resp.Code == uint32(0) {
			break
		}
		if attempt < 3 {
			f.t.Logf("Transaction broadcast failed (attempt %d/3): %v, retrying...", attempt, err)
			time.Sleep(time.Duration(attempt) * 2 * time.Second)
		}
	}
	require.NoError(f.t, err, "failed to broadcast funding transaction")
	require.Equal(f.t, resp.Code, uint32(0), "transaction failed with code %d", resp.Code)

	waitCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Wait for transaction confirmation
	for retry := 0; retry < 3; retry++ {
		err = wait.ForBlocks(waitCtx, 3, f.celestia)
		if err == nil {
			break
		}
		if retry < 2 {
			f.t.Logf("Failed to wait for blocks (attempt %d/3): %v, retrying...", retry+1, err)
			time.Sleep(2 * time.Second)
		}
	}
	require.NoError(f.t, err, "failed to wait for blocks after funding transaction")

	// Verify funding was successful
	for attempt := 1; attempt <= 5; attempt++ {
		bal := f.queryBalance(waitCtx, toAddr.String())
		if bal.Amount.Int64() >= amount {
			f.t.Logf("Successfully verified funding: %d utia transferred to %s", bal.Amount.Int64(), toAddr.String())
			return
		}
		if attempt < 5 {
			f.t.Logf("Balance check attempt %d/5: expected %d utia, got %d utia, retrying...",
				attempt, amount, bal.Amount.Int64())
			time.Sleep(2 * time.Second)
		}
	}

	bal := f.queryBalance(waitCtx, toAddr.String())
	require.GreaterOrEqual(f.t, bal.Amount.Int64(), amount, "balance is not sufficient after funding")
}

// verifyNodeBalance verifies that a DA node has the expected balance after funding.
func (f *Framework) verifyNodeBalance(ctx context.Context, daNode *dataavailability.Node, expectedAmount int64, nodeType string) {
	nodeClient := f.GetNodeRPCClient(ctx, daNode)
	nodeAddr, err := nodeClient.State.AccountAddress(ctx)
	require.NoError(f.t, err, "failed to get %s account address", nodeType)

	maxRetries := 5
	for attempt := 1; attempt <= maxRetries; attempt++ {
		bal := f.queryBalance(ctx, nodeAddr.String())
		if bal.Amount.Int64() >= expectedAmount {
			f.t.Logf("Successfully verified %s balance: %d utia (expected: %d)", nodeType, bal.Amount.Int64(), expectedAmount)
			return
		}

		if attempt < maxRetries {
			f.t.Logf("Balance check attempt %d/%d for %s: expected %d utia, got %d utia, retrying...",
				attempt, maxRetries, nodeType, expectedAmount, bal.Amount.Int64())
			time.Sleep(2 * time.Second)
		}
	}

	bal := f.queryBalance(ctx, nodeAddr.String())
	// Allow small tolerance for gas fees
	minExpected := int64(float64(expectedAmount) * 0.95)
	require.GreaterOrEqual(f.t, bal.Amount.Int64(), minExpected,
		"%s should have sufficient balance after funding (got %d, expected at least %d)",
		nodeType, bal.Amount.Int64(), minExpected)
}

// fundNodeAccount funds a specific DA node account using the default wallet.
func (f *Framework) fundNodeAccount(ctx context.Context, daNode *dataavailability.Node, amount int64) {
	fundingWallet := f.getOrCreateFundingWallet(ctx)
	nodeClient := f.GetNodeRPCClient(ctx, daNode)

	// Get the node's account address with retry logic
	var nodeAddr state.Address
	var err error
	for attempt := 1; attempt <= 3; attempt++ {
		nodeAddr, err = nodeClient.State.AccountAddress(ctx)
		if err == nil {
			break
		}
		if attempt < 3 {
			f.t.Logf("Failed to get node account address (attempt %d/3): %v, retrying...", attempt, err)
			time.Sleep(2 * time.Second)
		}
	}
	require.NoError(f.t, err, "failed to get node account address")

	nodeAccAddr := sdk.AccAddress(nodeAddr.Bytes())

	current := f.queryBalance(ctx, nodeAccAddr.String())
	if current.Amount.Int64() >= amount {
		f.t.Logf("Skipping funding for %s: current balance %d >= %d", nodeAccAddr.String(), current.Amount.Int64(), amount)
		return
	}

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

	chainBuilder := cosmos.NewChainBuilderWithTestName(f.t, fmt.Sprintf("%s-%d", f.t.Name(), time.Now().UnixNano())).
		WithDockerClient(f.client).
		WithDockerNetworkID(f.network).
		WithImage(chainImage).
		WithEncodingConfig(&enc).
		WithAdditionalStartArgs(
			"--force-no-bbr",
			"--grpc.enable",
			"--grpc.address", "0.0.0.0:9090",
			"--rpc.grpc_laddr", "tcp://0.0.0.0:9098",
			"--timeout-commit", "1s",
		).
		WithPostInit(func(ctx context.Context, node *cosmos.ChainNode) error {
			if err := config.Modify(ctx, node, "config/config.toml", func(cfg *cometcfg.Config) {
				cfg.TxIndex.Indexer = "kv"
			}); err != nil {
				return err
			}
			return config.Modify(ctx, node, "config/app.toml", func(cfg *servercfg.Config) {
				cfg.GRPC.Enable = true
				// Set minimum gas price for worker account compatibility
				cfg.MinGasPrices = "0.004utia"
			})
		})

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

	// always have at least one bridge node.
	bridgeNodeConfig := dataavailability.NewNodeBuilder().
		WithNodeType(types.BridgeNode).
		Build()

	daNetworkBuilder := dataavailability.NewNetworkBuilderWithTestName(f.t, fmt.Sprintf("%s-%d", f.t.Name(), time.Now().UnixNano())).
		WithDockerClient(f.client).
		WithDockerNetworkID(f.network).
		WithImage(daImage).
		WithNodes(bridgeNodeConfig).
		WithEnv("CELESTIA_KEYRING_BACKEND", "memory").
		WithEnv("CELESTIA_NODE_KEY", "test-key-mnemonic")

	return chainBuilder, daNetworkBuilder
}

// createAndStartCelestiaChain initializes and starts the Celestia chain.
func (f *Framework) createAndStartCelestiaChain(ctx context.Context) *cosmos.Chain {
	celestia, err := f.chainBuilder.Build(ctx)
	require.NoError(f.t, err, "failed to build celestia chain")
	err = celestia.Start(ctx)
	require.NoError(f.t, err, "failed to start celestia chain")

	require.NoError(f.t, wait.ForBlocks(ctx, 2, celestia))
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

	// Build start arguments
	startArgs := []string{"--p2p.network", testChainID, "--core.ip", hostname, "--rpc.addr", "0.0.0.0", "--keyring.backend", "test"}
	if f.config.TxWorkerAccounts > 0 {
		startArgs = append(startArgs, "--tx.worker.accounts", fmt.Sprintf("%d", f.config.TxWorkerAccounts))
	}

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

	// Start light node with retry logic
	for attempt := 1; attempt <= 2; attempt++ {
		err = lightNode.Start(ctx,
			dataavailability.WithChainID(testChainID),
			dataavailability.WithAdditionalStartArguments("--p2p.network", testChainID, "--core.ip", hostname, "--rpc.addr", "0.0.0.0"),
			dataavailability.WithEnvironmentVariables(
				map[string]string{
					"CELESTIA_CUSTOM": types.BuildCelestiaCustomEnvVar(testChainID, genesisHash, p2pAddr),
					"P2P_NETWORK":     testChainID,
				},
			),
		)
		if err == nil {
			return lightNode
		}
		if errdefs.IsNotFound(err) && attempt < 2 {
			f.t.Logf("retrying light node start due to missing container (attempt %d/2): %v", attempt, err)
			time.Sleep(2 * time.Second)
			continue
		}
		require.NoError(f.t, err, "failed to start light node")
	}
	return nil
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
	return defaultCelestiaTag
}

// getNodeTag returns the Celestia node image tag.
func getNodeTag() string {
	if tag := os.Getenv("CELESTIA_NODE_TAG"); tag != "" {
		return tag
	}
	return defaultNodeTag
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

	// Stop Celestia chain
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

	f.logger.Info("Framework cleanup completed")
	return nil
}
