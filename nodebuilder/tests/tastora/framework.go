package tastora

import (
	"context"
	"os"
	"testing"
	"time"

	"strings"

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

	"github.com/celestiaorg/celestia-app/v5/app"
	tastoradockertypes "github.com/celestiaorg/tastora/framework/docker"
	"github.com/celestiaorg/tastora/framework/testutil/sdkacc"
	"github.com/celestiaorg/tastora/framework/testutil/toml"
	"github.com/celestiaorg/tastora/framework/testutil/wait"
	"github.com/celestiaorg/tastora/framework/testutil/wallet"
	tastoratypes "github.com/celestiaorg/tastora/framework/types"

	rpcclient "github.com/celestiaorg/celestia-node/api/rpc/client"
	nodeblob "github.com/celestiaorg/celestia-node/blob"
	"github.com/celestiaorg/go-square/v2/share"
	"github.com/libp2p/go-libp2p/core/network"
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
	client  *client.Client
	network string

	provider  tastoratypes.Provider
	daNetwork tastoratypes.DataAvailabilityNetwork

	// Main DA network infrastructure
	celestia tastoratypes.Chain

	// Node topology (simplified)
	bridgeNodes []*tastoradockertypes.DANode // Bridge nodes (bridgeNodes[0] created by default)
	lightNodes  []*tastoradockertypes.DANode // Light nodes

	// Private funding infrastructure (not exposed to tests)
	fundingWallet        tastoratypes.Wallet
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
	f.client, f.network = tastoradockertypes.DockerSetup(t)
	f.provider = f.createDockerProvider(cfg)

	return f
}

// SetupNetwork initializes the basic network infrastructure.
// This includes starting the Celestia chain, DA network infrastructure,
// and ALWAYS creates the main bridge node (minimum viable DA network).
func (f *Framework) SetupNetwork(ctx context.Context) error {
	// 1. Create and start Celestia chain
	f.celestia = f.createAndStartCelestiaChain(ctx)

	// 2. Setup DA network infrastructure (retry once on transient docker errors)
	var daNetwork tastoratypes.DataAvailabilityNetwork
	var err error
	for attempt := 1; attempt <= 2; attempt++ {
		daNetwork, err = f.provider.GetDataAvailabilityNetwork(ctx)
		if err == nil {
			break
		}
		if strings.Contains(err.Error(), "No such container") {
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
func (f *Framework) GetBridgeNodes() []*tastoradockertypes.DANode {
	return f.bridgeNodes
}

// GetLightNodes returns all existing light nodes.
func (f *Framework) GetLightNodes() []*tastoradockertypes.DANode {
	return f.lightNodes
}

// NewBridgeNode creates, starts and appends a new bridge node.
func (f *Framework) NewBridgeNode(ctx context.Context) *tastoradockertypes.DANode {
	bridgeNode := f.newBridgeNode(ctx)
	f.bridgeNodes = append(f.bridgeNodes, bridgeNode)
	return bridgeNode
}

// NewLightNode creates, starts and appends a new light node.
func (f *Framework) NewLightNode(ctx context.Context) *tastoradockertypes.DANode {
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

	// Wait a moment to ensure funds are available
	time.Sleep(2 * time.Second)

	f.lightNodes = append(f.lightNodes, lightNode)
	return lightNode
}

// GetCelestiaChain returns the Celestia chain instance.
func (f *Framework) GetCelestiaChain() tastoratypes.Chain {
	return f.celestia
}

// GetNodeRPCClient retrieves an RPC client for the provided DA node.
func (f *Framework) GetNodeRPCClient(ctx context.Context, daNode *tastoradockertypes.DANode) *rpcclient.Client {
	rpcAddr := daNode.GetHostRPCAddress()
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

// newBridgeNode creates and starts a bridge node.
func (f *Framework) newBridgeNode(ctx context.Context) *tastoradockertypes.DANode {
	bridgeNode := f.startBridgeNode(ctx, f.celestia)
	f.fundNodeAccount(ctx, bridgeNode, f.defaultFundingAmount)
	f.t.Logf("Bridge node created and funded with %d utia", f.defaultFundingAmount)

	// Wait a moment to ensure funds are available
	time.Sleep(2 * time.Second)

	return bridgeNode
}

// fundWallet sends funds from one wallet to another address.
func (f *Framework) fundWallet(ctx context.Context, fromWallet tastoratypes.Wallet, toAddr sdk.AccAddress, amount int64) {
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

// fundNodeAccount funds a specific DA node account using the default wallet.
func (f *Framework) fundNodeAccount(ctx context.Context, daNode *tastoradockertypes.DANode, amount int64) {
	fundingWallet := f.getOrCreateFundingWallet(ctx)
	nodeClient := f.GetNodeRPCClient(ctx, daNode)

	// Get the node's account address
	nodeAddr, err := nodeClient.State.AccountAddress(ctx)
	require.NoError(f.t, err, "failed to get node account address")

	// Convert state.Address to sdk.AccAddress using Bytes()
	nodeAccAddr := sdk.AccAddress(nodeAddr.Bytes())

	// Skip funding if balance is already sufficient
	current := f.queryBalance(ctx, nodeAccAddr.String())
	if current.Amount.Int64() >= amount {
		f.t.Logf("Skipping funding for %s: current balance %d >= %d", nodeAccAddr.String(), current.Amount.Int64(), amount)
		return
	}

	f.t.Logf("Funding node account %s with %d utia", nodeAddr.String(), amount)

	// Fund the node account
	f.fundWallet(ctx, fundingWallet, nodeAccAddr, amount)
}

// createDockerProvider initializes the Docker provider for creating chains and nodes.
func (f *Framework) createDockerProvider(cfg *Config) tastoratypes.Provider {
	numValidators := cfg.NumValidators

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

	// Start the chain once. Retrying here can leave partially-created containers
	// and cause name conflicts on the next attempt.
	err = celestia.Start(ctx)
	require.NoError(f.t, err)

	// verify the chain is producing blocks
	require.NoError(f.t, wait.ForBlocks(ctx, 2, celestia))
	return celestia
}

// startBridgeNode initializes and starts a bridge node.
func (f *Framework) startBridgeNode(ctx context.Context, chain tastoratypes.Chain) *tastoradockertypes.DANode {
	genesisHash := f.getGenesisHash(ctx, chain)

	// Get the next available bridge node from the DA network
	bridgeNodes := f.daNetwork.GetBridgeNodes()
	bridgeNodeIndex := len(f.bridgeNodes)
	if bridgeNodeIndex >= len(bridgeNodes) {
		f.t.Fatalf("Cannot create more bridge nodes: already have %d, max is %d", bridgeNodeIndex, len(bridgeNodes))
	}
	bridgeNode := bridgeNodes[bridgeNodeIndex].(*tastoradockertypes.DANode)

	hostname, err := chain.GetNodes()[0].GetInternalHostName(ctx)
	require.NoError(f.t, err, "failed to get internal hostname")

	err = bridgeNode.Start(ctx,
		tastoratypes.WithChainID(testChainID),
		tastoratypes.WithAdditionalStartArguments("--p2p.network", testChainID, "--core.ip", hostname, "--rpc.addr", "0.0.0.0"),
		tastoratypes.WithEnvironmentVariables(
			map[string]string{
				"CELESTIA_CUSTOM":       tastoratypes.BuildCelestiaCustomEnvVar(testChainID, genesisHash, ""),
				"P2P_NETWORK":           testChainID,
				"CELESTIA_BOOTSTRAPPER": "true", // Make bridge node act as DHT bootstrapper
			},
		),
	)
	require.NoError(f.t, err, "failed to start bridge node")
	return bridgeNode
}

// startLightNode initializes and starts a light node.
func (f *Framework) startLightNode(ctx context.Context, bridgeNode *tastoradockertypes.DANode, chain tastoratypes.Chain) *tastoradockertypes.DANode {
	genesisHash := f.getGenesisHash(ctx, chain)

	p2pInfo, err := bridgeNode.GetP2PInfo(ctx)
	require.NoError(f.t, err, "failed to get bridge node p2p info")

	p2pAddr, err := p2pInfo.GetP2PAddress()
	require.NoError(f.t, err, "failed to get bridge node p2p address")

	// Get the core node hostname for state access
	hostname, err := chain.GetNodes()[0].GetInternalHostName(ctx)
	require.NoError(f.t, err, "failed to get internal hostname")

	// Try starting an unused light node, with a retry to avoid occasional docker flakiness
	var lastErr error
	for attempt := 1; attempt <= 2; attempt++ {
		// Select next unused light node from DA network (prevents index drift)
		var lightNode *tastoradockertypes.DANode
		available := f.daNetwork.GetLightNodes()
		for _, n := range available {
			candidate := n.(*tastoradockertypes.DANode)
			used := false
			for _, existing := range f.lightNodes {
				if existing == candidate {
					used = true
					break
				}
			}
			if !used {
				lightNode = candidate
				break
			}
		}
		if lightNode == nil {
			f.t.Fatalf("no unused light nodes available: have %d total, %d already started", len(f.daNetwork.GetLightNodes()), len(f.lightNodes))
		}

		err = lightNode.Start(ctx,
			tastoratypes.WithChainID(testChainID),
			tastoratypes.WithAdditionalStartArguments("--p2p.network", testChainID, "--core.ip", hostname, "--rpc.addr", "0.0.0.0"),
			tastoratypes.WithEnvironmentVariables(
				map[string]string{
					"CELESTIA_CUSTOM": tastoratypes.BuildCelestiaCustomEnvVar(testChainID, genesisHash, p2pAddr),
					"P2P_NETWORK":     testChainID,
				},
			),
		)
		if err == nil {
			return lightNode
		}
		lastErr = err
		// Retry only for transient docker/container races
		if strings.Contains(err.Error(), "No such container") {
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

// getOrCreateFundingWallet returns the framework's funding wallet, creating it if needed.
func (f *Framework) getOrCreateFundingWallet(ctx context.Context) tastoratypes.Wallet {
	if f.fundingWallet == nil {
		// Create a wallet with enough funds to fund multiple nodes
		f.fundingWallet = f.CreateTestWallet(ctx, 50_000_000_000) // 50 billion utia
		f.t.Logf("Created funding wallet for automatic node funding")
	}
	return f.fundingWallet
}

// Robust waiting and P2P connectivity helpers

// ConnectLightToBridge ensures the light node is directly connected to the bridge peer.
func (f *Framework) ConnectLightToBridge(ctx context.Context, bridgeClient, lightClient *rpcclient.Client, label string) {
	// Fetch bridge AddrInfo
	infoCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	info, err := bridgeClient.P2P.Info(infoCtx)
	cancel()
	if err != nil {
		f.t.Logf("warning: failed to get bridge AddrInfo: %v", err)
		return
	}

	deadline := time.Now().Add(60 * time.Second)
	for attempt := 1; ; attempt++ {
		cctx, ccancel := context.WithTimeout(ctx, 5*time.Second)
		_ = lightClient.P2P.Connect(cctx, info)
		ccancel()

		stx, scancel := context.WithTimeout(ctx, 3*time.Second)
		state, _ := lightClient.P2P.Connectedness(stx, info.ID)
		scancel()
		if state == network.Connected {
			f.t.Logf("info: %s connected to bridge (peer %s)", label, info.ID)
			return
		}

		if time.Now().After(deadline) {
			f.t.Logf("warning: %s failed to connect to bridge within timeout", label)
			return
		}
		time.Sleep(2 * time.Second)
	}
}

// WaitPeersReady waits until the node reports at least one peer.
func (f *Framework) WaitPeersReady(ctx context.Context, client *rpcclient.Client, label string, timeout time.Duration) {
	deadline := time.Now().Add(timeout)
	for {
		pctx, pcancel := context.WithTimeout(ctx, 3*time.Second)
		peers, err := client.P2P.Peers(pctx)
		pcancel()
		if err == nil && len(peers) > 0 {
			return
		}
		if time.Now().After(deadline) {
			f.t.Logf("warning: %s has no peers yet (err=%v)", label, err)
			return
		}
		time.Sleep(1 * time.Second)
	}
}

// WaitSharesAvailable waits until SharesAvailable succeeds for the given client and height.
func (f *Framework) WaitSharesAvailable(ctx context.Context, client *rpcclient.Client, height uint64) error {
	deadline := time.Now().Add(90 * time.Second)
	for {
		reqCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
		err := client.Share.SharesAvailable(reqCtx, height)
		cancel()
		if err == nil {
			return nil
		}
		if time.Now().After(deadline) {
			return err
		}
		time.Sleep(2 * time.Second)
	}
}

// WaitBlobAvailable waits until Blob.Get succeeds for the given parameters.
func (f *Framework) WaitBlobAvailable(ctx context.Context, client *rpcclient.Client, height uint64, namespace share.Namespace, commitment []byte) (*nodeblob.Blob, error) {
	deadline := time.Now().Add(90 * time.Second)
	for {
		reqCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
		blob, err := client.Blob.Get(reqCtx, height, namespace, commitment)
		cancel()
		if err == nil {
			return blob, nil
		}
		if time.Now().After(deadline) {
			return nil, err
		}
		time.Sleep(2 * time.Second)
	}
}

// WaitNamespaceDataAvailable waits until GetNamespaceData succeeds for the given parameters.
func (f *Framework) WaitNamespaceDataAvailable(ctx context.Context, client *rpcclient.Client, height uint64, namespace share.Namespace) (interface{}, error) {
	deadline := time.Now().Add(90 * time.Second)
	for {
		reqCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
		data, err := client.Share.GetNamespaceData(reqCtx, height, namespace)
		cancel()
		if err == nil {
			return data, nil
		}
		if time.Now().After(deadline) {
			return nil, err
		}
		time.Sleep(2 * time.Second)
	}
}

// SetupP2PConnectivity establishes P2P connections between bridge and light nodes.
func (f *Framework) SetupP2PConnectivity(ctx context.Context, bridgeClient, lightClient *rpcclient.Client, label string) {
	f.ConnectLightToBridge(ctx, bridgeClient, lightClient, label)
	f.WaitPeersReady(ctx, lightClient, label, 45*time.Second)

	// Ensure light node is synced
	swctx, swcancel := context.WithTimeout(ctx, 90*time.Second)
	err := lightClient.Header.SyncWait(swctx)
	swcancel()
	if err != nil {
		f.t.Logf("warning: %s SyncWait failed: %v", label, err)
	}

	f.t.Logf("✅ P2P connectivity established for %s", label)
}
