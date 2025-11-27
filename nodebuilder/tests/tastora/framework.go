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
	defaultNodeTag = "37c99f2" // fallback if not set via ldflags
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
}

// NewFramework creates a new Tastora testing framework instance.
// Similar to swamp.NewSwamp(), this sets up the complete testing environment.
func NewFramework(t *testing.T, options ...Option) *Framework {
	f := &Framework{
		t:                    t,
		logger:               zaptest.NewLogger(t),
		defaultFundingAmount: 10_000_000_000, // 10 billion utia - enough for multiple blob transactions
	}

	// Apply configuration options
	cfg := defaultConfig()
	for _, opt := range options {
		opt(cfg)
	}

	f.logger.Info("Setting up Tastora framework", zap.String("test", t.Name()))
	f.client, f.network = docker.Setup(t)
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

	grpcCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
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
	require.NoError(f.t, err, "failed to query balance")
	return *res.Balance
}

// newBridgeNode creates and starts a bridge node.
func (f *Framework) newBridgeNode(ctx context.Context) *dataavailability.Node {
	bridgeNode := f.startBridgeNode(ctx, f.celestia)
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
	resp, err := f.celestia.BroadcastMessages(ctx, fromWallet, bankSend)
	require.NoError(f.t, err)
	require.Equal(f.t, resp.Code, uint32(0), "resp: %v", resp)

	waitCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	maxRetries := 3
	for retry := 0; retry < maxRetries; retry++ {
		err = wait.ForBlocks(waitCtx, 3, f.celestia) // Increased from 2 to 3 blocks
		if err == nil {
			break
		}
		if retry < maxRetries-1 {
			f.t.Logf("Failed to wait for blocks (attempt %d/%d): %v, retrying...", retry+1, maxRetries, err)
			time.Sleep(2 * time.Second)
		}
	}
	require.NoError(f.t, err, "failed to wait for blocks after funding transaction")

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
// This function provides deterministic balance verification with retries and waiting.
// It should be called after fundNodeAccount to ensure funding was successful.
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
	require.GreaterOrEqual(f.t, bal.Amount.Int64(), expectedAmount,
		"%s should have sufficient balance after funding (final check: got %d, expected %d)",
		nodeType, bal.Amount.Int64(), expectedAmount)
}

// fundNodeAccount funds a specific DA node account using the default wallet.
func (f *Framework) fundNodeAccount(ctx context.Context, daNode *dataavailability.Node, amount int64) {
	fundingWallet := f.getOrCreateFundingWallet(ctx)
	nodeClient := f.GetNodeRPCClient(ctx, daNode)

	// Get the node's account address
	nodeAddr, err := nodeClient.State.AccountAddress(ctx)
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

	chainBuilder := cosmos.NewChainBuilderWithTestName(f.t, f.t.Name()).
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

	daNetworkBuilder := dataavailability.NewNetworkBuilderWithTestName(f.t, f.t.Name()).
		WithDockerClient(f.client).
		WithDockerNetworkID(f.network).
		WithImage(daImage).
		WithNodes(nodeConfigs...)

	return chainBuilder, daNetworkBuilder
}

// createAndStartCelestiaChain initializes and starts the Celestia chain.
func (f *Framework) createAndStartCelestiaChain(ctx context.Context) *cosmos.Chain {
	celestia, err := f.chainBuilder.Build(ctx)
	require.NoError(f.t, err, "failed to build celestia chain")
	f.celestia = celestia
	err = f.celestia.Start(ctx)
	require.NoError(f.t, err, "failed to start celestia chain")

	require.NoError(f.t, wait.ForBlocks(ctx, 2, f.celestia))
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

	err = bridgeNode.Start(ctx,
		dataavailability.WithChainID(testChainID),
		dataavailability.WithAdditionalStartArguments("--p2p.network", testChainID, "--core.ip", hostname, "--rpc.addr", "0.0.0.0"),
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

	// Try starting the light node, with a retry to avoid occasional docker flakiness
	var lastErr error
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
		lastErr = err
		// Retry only for transient docker/container races
		if errdefs.IsNotFound(err) {
			f.t.Logf("retrying light node start due to missing container (attempt %d/2): %v", attempt, err)
			time.Sleep(2 * time.Second)
			continue
		}
		require.NoError(f.t, err, "failed to start light node")
	}
	require.NoError(f.t, lastErr, "failed to start light node after retries")
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
		f.fundingWallet = f.CreateTestWallet(ctx, 50_000_000_000)
		f.t.Logf("Created funding wallet for automatic node funding")
	}
	return f.fundingWallet
}
