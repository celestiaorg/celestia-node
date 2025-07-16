package tastora

import (
	"context"
	"os"
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

	"github.com/celestiaorg/celestia-app/v4/app"
	tastoradockertypes "github.com/celestiaorg/tastora/framework/docker"
	"github.com/celestiaorg/tastora/framework/testutil/sdkacc"
	"github.com/celestiaorg/tastora/framework/testutil/toml"
	"github.com/celestiaorg/tastora/framework/testutil/wait"
	"github.com/celestiaorg/tastora/framework/testutil/wallet"
	tastoratypes "github.com/celestiaorg/tastora/framework/types"

	rpcclient "github.com/celestiaorg/celestia-node/api/rpc/client"
)

const (
	celestiaAppImage   = "ghcr.io/celestiaorg/celestia-app"
	defaultCelestiaTag = "v4.0.0-rc6"
	nodeImage          = "ghcr.io/celestiaorg/celestia-node"
	defaultNodeTag     = "v0.23.2-mocha"
	testChainID        = "test"
)

// Framework represents the main testing infrastructure for Tastora-based tests.
// It provides Docker-based chain and node setup, similar to how Swamp provides
// mock-based testing infrastructure.
type Framework struct {
	t       *testing.T
	logger  *zap.Logger
	client  *client.Client
	network string

	provider   tastoratypes.Provider
	daNetwork  tastoratypes.DataAvailabilityNetwork
	bridgeNode tastoratypes.DANode
	fullNodes  []tastoratypes.DANode
	lightNodes []tastoratypes.DANode
	celestia   tastoratypes.Chain

	// Default wallet for automatic node funding
	defaultWallet tastoratypes.Wallet
	// Default funding amount for nodes (3 billion utia)
	defaultFundingAmount int64
}

// NewFramework creates a new Tastora testing framework instance.
// Similar to swamp.NewSwamp(), this sets up the complete testing environment.
func NewFramework(t *testing.T, options ...Option) *Framework {
	f := &Framework{
		t:                    t,
		logger:               zaptest.NewLogger(t),
		defaultFundingAmount: 3_000_000_000, // 3 billion utia - default funding amount
	}

	// Apply configuration options
	cfg := defaultConfig()
	for _, opt := range options {
		opt(cfg)
	}

	f.logger.Info("Setting up Tastora framework", zap.String("test", t.Name()))
	f.client, f.network = tastoradockertypes.DockerSetup(t)
	f.provider = f.createDockerProvider(cfg)

	return f
}

// SetupNetwork initializes the basic network infrastructure.
// This includes starting the Celestia chain and DA network infrastructure.
// Nodes are created lazily when requested via GetOrCreate/New methods.
func (f *Framework) SetupNetwork(ctx context.Context) error {
	f.celestia = f.createAndStartCelestiaChain(ctx)

	daNetwork, err := f.provider.GetDataAvailabilityNetwork(ctx)
	if err != nil {
		return err
	}
	f.daNetwork = daNetwork

	// Don't create any nodes by default - let tests create them as needed
	// This provides better resource usage and more flexible testing

	return nil
}

// GetBridgeNode returns the bridge node instance.
func (f *Framework) GetBridgeNode() tastoratypes.DANode {
	return f.bridgeNode
}

// GetOrCreateBridgeNode returns the bridge node, creating it if it doesn't exist.
// The bridge node is automatically funded with the default amount for transaction operations.
func (f *Framework) GetOrCreateBridgeNode(ctx context.Context) tastoratypes.DANode {
	if f.bridgeNode == nil {
		f.bridgeNode = f.startBridgeNode(ctx, f.celestia)

		// Automatically fund the bridge node
		defaultWallet := f.getOrCreateDefaultWallet(ctx)
		f.FundNodeAccount(ctx, defaultWallet, f.bridgeNode, f.defaultFundingAmount)
		f.t.Logf("Bridge node automatically funded with %d utia", f.defaultFundingAmount)

		// Wait a moment to ensure funds are available
		time.Sleep(2 * time.Second)
	}
	return f.bridgeNode
}

// NewFullNode creates and starts a new full node.
// The full node is automatically funded with the default amount for transaction operations.
func (f *Framework) NewFullNode(ctx context.Context) tastoratypes.DANode {
	// Ensure we have a bridge node to connect to
	bridgeNode := f.GetOrCreateBridgeNode(ctx)

	// Get the next available full node from the DA network
	allFullNodes := f.daNetwork.GetFullNodes()
	if len(f.fullNodes) >= len(allFullNodes) {
		f.t.Fatalf("Cannot create more full nodes: already have %d, max is %d", len(f.fullNodes), len(allFullNodes))
	}

	fullNode := f.startFullNode(ctx, bridgeNode, f.celestia)

	// Automatically fund the full node
	defaultWallet := f.getOrCreateDefaultWallet(ctx)
	f.FundNodeAccount(ctx, defaultWallet, fullNode, f.defaultFundingAmount)
	f.t.Logf("Full node automatically funded with %d utia", f.defaultFundingAmount)

	// Wait a moment to ensure funds are available
	time.Sleep(2 * time.Second)

	f.fullNodes = append(f.fullNodes, fullNode)
	return fullNode
}

// NewLightNode creates and starts a new light node.
// The light node is automatically funded with the default amount for transaction operations.
func (f *Framework) NewLightNode(ctx context.Context) tastoratypes.DANode {
	// Ensure we have a full node to connect to
	var fullNode tastoratypes.DANode
	if len(f.fullNodes) == 0 {
		fullNode = f.NewFullNode(ctx)
	} else {
		fullNode = f.fullNodes[0]
	}

	// Get the next available light node from the DA network
	allLightNodes := f.daNetwork.GetLightNodes()
	if len(f.lightNodes) >= len(allLightNodes) {
		f.t.Fatalf("Cannot create more light nodes: already have %d, max is %d", len(f.lightNodes), len(allLightNodes))
	}

	lightNode := f.startLightNode(ctx, fullNode, f.celestia)

	// Automatically fund the light node
	defaultWallet := f.getOrCreateDefaultWallet(ctx)
	f.FundNodeAccount(ctx, defaultWallet, lightNode, f.defaultFundingAmount)
	f.t.Logf("Light node automatically funded with %d utia", f.defaultFundingAmount)

	// Wait a moment to ensure funds are available
	time.Sleep(2 * time.Second)

	f.lightNodes = append(f.lightNodes, lightNode)
	return lightNode
}

// GetBridgeNodes returns all bridge node instances.
func (f *Framework) GetBridgeNodes() []tastoratypes.DANode {
	if f.bridgeNode != nil {
		return []tastoratypes.DANode{f.bridgeNode}
	}
	return []tastoratypes.DANode{}
}

// GetOrCreateFullNode returns the first full node, creating it if none exist.
func (f *Framework) GetOrCreateFullNode(ctx context.Context) tastoratypes.DANode {
	if len(f.fullNodes) == 0 {
		return f.NewFullNode(ctx)
	}
	return f.fullNodes[0]
}

// GetOrCreateLightNode returns the first light node, creating it if none exist.
func (f *Framework) GetOrCreateLightNode(ctx context.Context) tastoratypes.DANode {
	if len(f.lightNodes) == 0 {
		return f.NewLightNode(ctx)
	}
	return f.lightNodes[0]
}

// GetFullNodes returns all full node instances.
func (f *Framework) GetFullNodes() []tastoratypes.DANode {
	return f.fullNodes
}

// GetLightNodes returns all light node instances.
func (f *Framework) GetLightNodes() []tastoratypes.DANode {
	return f.lightNodes
}

// GetCelestiaChain returns the Celestia chain instance.
func (f *Framework) GetCelestiaChain() tastoratypes.Chain {
	return f.celestia
}

// GetNodeRPCClient retrieves an RPC client for the provided DA node.
func (f *Framework) GetNodeRPCClient(ctx context.Context, daNode tastoratypes.DANode) *rpcclient.Client {
	rpcAddr := daNode.GetHostRPCAddress()
	require.NotEmpty(f.t, rpcAddr, "rpc address is empty")

	rpcClient, err := rpcclient.NewClient(ctx, "http://"+rpcAddr, "")
	require.NoError(f.t, err)
	return rpcClient
}

// CreateTestWallet creates a new test wallet on the chain, funding it with the specified amount.
func (f *Framework) CreateTestWallet(ctx context.Context, amount int64) tastoratypes.Wallet {
	sendAmount := sdk.NewCoins(sdk.NewCoin("utia", sdkmath.NewInt(amount)))
	testWallet, err := wallet.CreateAndFund(ctx, "test", sendAmount, f.celestia)
	require.NoError(f.t, err, "failed to create test wallet")
	require.NotNil(f.t, testWallet, "wallet is nil")
	return testWallet
}

// queryBalance fetches the balance of a given address.
func (f *Framework) queryBalance(ctx context.Context, addr string) sdk.Coin {
	grpcAddr := f.celestia.GetGRPCAddress()

	// Use a timeout context for GRPC connection
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

	// Use the timeout context for the balance query
	res, err := bankClient.Balance(grpcCtx, req)
	require.NoError(f.t, err, "failed to query balance")
	return *res.Balance
}

// FundWallet sends funds from one wallet to another address.
func (f *Framework) FundWallet(ctx context.Context, fromWallet tastoratypes.Wallet, toAddr sdk.AccAddress, amount int64) {
	fromAddr, err := sdkacc.AddressFromWallet(fromWallet)
	require.NoError(f.t, err, "failed to get from address")

	f.t.Logf("sending funds from %s to %s", fromAddr.String(), toAddr.String())

	bankSend := banktypes.NewMsgSend(fromAddr, toAddr.Bytes(), sdk.NewCoins(sdk.NewCoin("utia", sdkmath.NewInt(amount))))
	resp, err := f.celestia.BroadcastMessages(ctx, fromWallet, bankSend)
	require.NoError(f.t, err)
	require.Equal(f.t, resp.Code, uint32(0), "resp: %v", resp)

	// Use a longer timeout context for waiting for blocks
	waitCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// wait for blocks to ensure the funds are available with retries
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

	// Check balance with retries
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

	// Final balance check
	bal := f.queryBalance(waitCtx, toAddr.String())
	require.GreaterOrEqual(f.t, bal.Amount.Int64(), amount, "balance is not sufficient after funding")
}

// FundNodeAccount funds a specific DA node account using the provided wallet.
func (f *Framework) FundNodeAccount(ctx context.Context, fromWallet tastoratypes.Wallet, daNode tastoratypes.DANode, amount int64) {
	nodeClient := f.GetNodeRPCClient(ctx, daNode)

	// Get the node's account address
	nodeAddr, err := nodeClient.State.AccountAddress(ctx)
	require.NoError(f.t, err, "failed to get node account address")

	f.t.Logf("Funding node account %s with %d utia", nodeAddr.String(), amount)

	// Convert state.Address to sdk.AccAddress using Bytes()
	nodeAccAddr := sdk.AccAddress(nodeAddr.Bytes())

	// Fund the node account
	f.FundWallet(ctx, fromWallet, nodeAccAddr, amount)
}

// createDockerProvider initializes the Docker provider for creating chains and nodes.
func (f *Framework) createDockerProvider(cfg *Config) tastoratypes.Provider {
	numValidators := cfg.NumValidators
	numFullNodes := cfg.NumFullNodes

	enc := testutil.MakeTestEncodingConfig(app.ModuleEncodingRegisters...)

	dockerCfg := tastoradockertypes.Config{
		Logger:          f.logger,
		DockerClient:    f.client,
		DockerNetworkID: f.network,
		ChainConfig: &tastoradockertypes.ChainConfig{
			ConfigFileOverrides: map[string]any{
				"config/app.toml":    appOverrides(),
				"config/config.toml": configOverrides(),
			},
			Type:          "cosmos",
			Name:          "celestia",
			Version:       getCelestiaTag(),
			NumValidators: &numValidators,
			NumFullNodes:  &numFullNodes,
			ChainID:       testChainID,
			Images: []tastoradockertypes.DockerImage{
				{
					Repository: celestiaAppImage,
					Version:    getCelestiaTag(),
					UIDGID:     "10001:10001",
				},
			},
			Bin:            "celestia-appd",
			Bech32Prefix:   "celestia",
			Denom:          "utia",
			CoinType:       "118",
			GasPrices:      "0.025utia",
			GasAdjustment:  1.3,
			EncodingConfig: &enc,
			AdditionalStartArgs: []string{
				"--force-no-bbr",
				"--grpc.enable",
				"--grpc.address", "0.0.0.0:9090",
				"--rpc.grpc_laddr", "tcp://0.0.0.0:9098",
				"--timeout-commit", "1s",
			},
		},
		DataAvailabilityNetworkConfig: &tastoradockertypes.DataAvailabilityNetworkConfig{
			FullNodeCount:   cfg.FullNodeCount,
			BridgeNodeCount: cfg.BridgeNodeCount,
			LightNodeCount:  cfg.LightNodeCount,
			Image: tastoradockertypes.DockerImage{
				Repository: getNodeImage(),
				Version:    getNodeTag(),
				UIDGID:     "10001:10001",
			},
		},
	}
	return tastoradockertypes.NewProvider(dockerCfg, f.t)
}

// createAndStartCelestiaChain initializes and starts the Celestia chain.
func (f *Framework) createAndStartCelestiaChain(ctx context.Context) tastoratypes.Chain {
	celestia, err := f.provider.GetChain(ctx)
	require.NoError(f.t, err, "failed to get chain")

	err = celestia.Start(ctx)
	require.NoError(f.t, err)

	// verify the chain is producing blocks
	require.NoError(f.t, wait.ForBlocks(ctx, 2, celestia))
	return celestia
}

// startBridgeNode initializes and starts a bridge node.
func (f *Framework) startBridgeNode(ctx context.Context, chain tastoratypes.Chain) tastoratypes.DANode {
	genesisHash := f.getGenesisHash(ctx, chain)

	// Get the first available bridge node from the DA network
	bridgeNodes := f.daNetwork.GetBridgeNodes()
	if len(bridgeNodes) == 0 {
		f.t.Fatalf("No bridge nodes available in DA network")
	}
	bridgeNode := bridgeNodes[0]

	hostname, err := chain.GetNodes()[0].GetInternalHostName(ctx)
	require.NoError(f.t, err, "failed to get internal hostname")

	err = bridgeNode.Start(ctx,
		tastoratypes.WithChainID(testChainID),
		tastoratypes.WithAdditionalStartArguments("--p2p.network", testChainID, "--core.ip", hostname, "--rpc.addr", "0.0.0.0"),
		tastoratypes.WithEnvironmentVariables(
			map[string]string{
				"CELESTIA_CUSTOM": tastoratypes.BuildCelestiaCustomEnvVar(testChainID, genesisHash, ""),
				"P2P_NETWORK":     testChainID,
			},
		),
	)
	require.NoError(f.t, err, "failed to start bridge node")
	return bridgeNode
}

// startFullNode initializes and starts a full node.
func (f *Framework) startFullNode(ctx context.Context, bridgeNode tastoratypes.DANode, chain tastoratypes.Chain) tastoratypes.DANode {
	genesisHash := f.getGenesisHash(ctx, chain)

	hostname, err := chain.GetNodes()[0].GetInternalHostName(ctx)
	require.NoError(f.t, err, "failed to get internal hostname")

	p2pInfo, err := bridgeNode.GetP2PInfo(ctx)
	require.NoError(f.t, err, "failed to get bridge node p2p info")

	p2pAddr, err := p2pInfo.GetP2PAddress()
	require.NoError(f.t, err, "failed to get bridge node p2p address")

	fullNode := f.daNetwork.GetFullNodes()[0]
	err = fullNode.Start(ctx,
		tastoratypes.WithChainID(testChainID),
		tastoratypes.WithAdditionalStartArguments("--p2p.network", testChainID, "--core.ip", hostname, "--rpc.addr", "0.0.0.0"),
		tastoratypes.WithEnvironmentVariables(
			map[string]string{
				"CELESTIA_CUSTOM": tastoratypes.BuildCelestiaCustomEnvVar(testChainID, genesisHash, p2pAddr),
				"P2P_NETWORK":     testChainID,
			},
		),
	)
	require.NoError(f.t, err, "failed to start full node")
	return fullNode
}

// startLightNode initializes and starts a light node.
func (f *Framework) startLightNode(ctx context.Context, fullNode tastoratypes.DANode, chain tastoratypes.Chain) tastoratypes.DANode {
	genesisHash := f.getGenesisHash(ctx, chain)

	p2pInfo, err := fullNode.GetP2PInfo(ctx)
	require.NoError(f.t, err, "failed to get full node p2p info")

	p2pAddr, err := p2pInfo.GetP2PAddress()
	require.NoError(f.t, err, "failed to get full node p2p address")

	lightNode := f.daNetwork.GetLightNodes()[0]
	err = lightNode.Start(ctx,
		tastoratypes.WithChainID(testChainID),
		tastoratypes.WithAdditionalStartArguments("--p2p.network", testChainID, "--rpc.addr", "0.0.0.0"),
		tastoratypes.WithEnvironmentVariables(
			map[string]string{
				"CELESTIA_CUSTOM": tastoratypes.BuildCelestiaCustomEnvVar(testChainID, genesisHash, p2pAddr),
				"P2P_NETWORK":     testChainID,
			},
		),
	)
	require.NoError(f.t, err, "failed to start light node")
	return lightNode
}

// getGenesisHash returns the genesis hash of the chain.
func (f *Framework) getGenesisHash(ctx context.Context, chain tastoratypes.Chain) string {
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

// appOverrides modifies the "app.toml" configuration.
func appOverrides() toml.Toml {
	appTomlOverride := make(toml.Toml)
	txIndexConfig := make(toml.Toml)
	txIndexConfig["indexer"] = "kv"
	appTomlOverride["tx-index"] = txIndexConfig
	return appTomlOverride
}

// configOverrides modifies the "config.toml" configuration.
func configOverrides() toml.Toml {
	overrides := make(toml.Toml)
	txIndexConfig := make(toml.Toml)
	txIndexConfig["indexer"] = "kv"
	overrides["tx_index"] = txIndexConfig
	return overrides
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

// getOrCreateDefaultWallet returns the default wallet, creating it if it doesn't exist.
// This wallet is used for automatic node funding.
func (f *Framework) getOrCreateDefaultWallet(ctx context.Context) tastoratypes.Wallet {
	if f.defaultWallet == nil {
		// Create a wallet with enough funds to fund multiple nodes
		f.defaultWallet = f.CreateTestWallet(ctx, 50_000_000_000) // 50 billion utia
		f.t.Logf("Created default wallet for automatic node funding")
	}
	return f.defaultWallet
}
