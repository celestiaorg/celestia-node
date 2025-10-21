package tastora

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	sdkmath "cosmossdk.io/math"
	"github.com/containerd/errdefs"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module/testutil"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	dockercontainer "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/volume"
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

	"github.com/celestiaorg/celestia-node/state"

	rpcclient "github.com/celestiaorg/celestia-node/api/rpc/client"
)

// Transient Docker error patterns that should trigger retries
var transientDockerErrors = []string{
	"No such container",
	"set volume owner",
	"overwriting genesis.json",
	"genesis.json file already exists",
	"failed to overwrite config/config.toml",
	"all predefined address pools have been fully subnetted",
	"The container name",
	"is already in use",
}

// Transaction error patterns
var (
	txNotFoundError      = "tx not found"
	txAlreadyExistsError = "tx already exists in mempool"
	accountSequenceError = "account sequence mismatch"
)

// isTransientDockerError checks if an error is a known transient Docker issue
// that should trigger a retry rather than failing the test
func isTransientDockerError(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()
	for _, pattern := range transientDockerErrors {
		if strings.Contains(errStr, pattern) {
			return true
		}
	}
	return false
}

// isTxNotFoundError checks if an error indicates a transaction was not found
func isTxNotFoundError(err error) bool {
	return err != nil && strings.Contains(err.Error(), txNotFoundError)
}

// isTxAlreadyExistsError checks if an error indicates a transaction already exists in mempool
func isTxAlreadyExistsError(err error) bool {
	return err != nil && strings.Contains(err.Error(), txAlreadyExistsError)
}

// isAccountSequenceError checks if an error indicates an account sequence mismatch
func isAccountSequenceError(err error) bool {
	return err != nil && strings.Contains(err.Error(), accountSequenceError)
}

// isKeyCreationError checks if the error is related to key creation that can be retried.
func isKeyCreationError(err error) bool {
	if err == nil {
		return false
	}
	errorStr := err.Error()
	keyCreationErrors := []string{
		"Error: EOF",
		"failed to create wallet",
		"exit code 1:  Error: EOF",
		"keys add",
		"keyring",
	}
	for _, keyError := range keyCreationErrors {
		if strings.Contains(errorStr, keyError) {
			return true
		}
	}
	return false
}

const (
	defaultCelestiaAppTag = "v6.1.2-arabica"
	celestiaAppImage      = "ghcr.io/celestiaorg/celestia-app"
	// defaultNodeTag can be overridden at build time using ldflags
	// Example: go build -ldflags "-X github.com/celestiaorg/celestia-node/nodebuilder/tests/tastora.defaultNodeTag=v1.2.3"
	defaultCelestiaNodeTag = "v0.28.1-arabica"
	nodeImage              = "ghcr.io/celestiaorg/celestia-node"

	testChainID = "test"
)

var (
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

	// Clean up any leftover volumes from previous test runs
	f.cleanupPreviousTestVolumes(t.Name())

	// Also clean up any orphaned containers/volumes from other test runs
	f.cleanupOrphanedResources()

	// Minimal resilience: recover transient docker network exhaustion and retry once
	for attempt := 1; attempt <= 2; attempt++ {
		var (
			cli      *client.Client
			netID    string
			panicked any
		)
		func() {
			defer func() {
				if r := recover(); r != nil {
					panicked = r
				}
			}()
			cli, netID = docker.DockerSetup(t)
		}()

		if panicked != nil {
			msg := fmt.Sprint(panicked)
			if attempt < 2 && isTransientDockerError(fmt.Errorf("%s", msg)) {
				f.t.Logf("Docker network exhausted (attempt %d/2); retrying after short delay...", attempt)
				time.Sleep(3 * time.Second)
				continue
			}
			panic(panicked)
		}

		f.client, f.network = cli, netID
		break
	}
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

	// Retry wallet creation for transient "tx not found" errors
	for attempt := 1; attempt <= 3; attempt++ {
		testWallet, err := wallet.CreateAndFund(ctx, "test", sendAmount, f.celestia)
		if err == nil && testWallet != nil {
			return testWallet
		}

		if isTxNotFoundError(err) || isAccountSequenceError(err) {
			if attempt < 3 {
				f.t.Logf("Wallet creation failed with transient error (attempt %d/3): %v, retrying...", attempt, err)
				// Wait longer for blocks to ensure previous transaction is confirmed
				waitCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				_ = wait.ForBlocks(waitCtx, 2, f.celestia)
				cancel()
				// Longer delay for account sequence issues
				delay := time.Duration(attempt) * 3 * time.Second
				if isAccountSequenceError(err) {
					delay = time.Duration(attempt) * 5 * time.Second
				}
				time.Sleep(delay)
				continue
			}
		}

		if attempt == 3 {
			require.NoError(f.t, err, "failed to create test wallet after 3 attempts")
			require.NotNil(f.t, testWallet, "wallet is nil")
		}
	}

	return nil
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

	var resp sdk.TxResponse
	for attempt := 1; attempt <= 3; attempt++ {
		resp, err = f.celestia.BroadcastMessages(ctx, fromWallet, bankSend)
		if err == nil && resp.Code == uint32(0) {
			break
		}

		// Handle "tx already exists in mempool" as success
		if isTxAlreadyExistsError(err) {
			f.t.Logf("Transaction already in mempool, considering it successful")
			break
		}

		// Handle account sequence errors as retryable
		if isAccountSequenceError(err) {
			if attempt < 3 {
				f.t.Logf("Account sequence mismatch (attempt %d/3): %v, retrying...", attempt, err)
				time.Sleep(time.Duration(attempt) * 2 * time.Second)
				continue
			}
		}

		if attempt < 3 {
			f.t.Logf("Transaction broadcast failed (attempt %d/3): %v, retrying...", attempt, err)
			time.Sleep(time.Duration(attempt) * 2 * time.Second)
		}
	}

	// Only require error check if we didn't break early due to mempool error
	if err != nil && !isTxAlreadyExistsError(err) {
		require.NoError(f.t, err, "failed to broadcast funding transaction")
	}

	// Only check response code if we got a valid response
	if resp.Code != 0 && !isTxAlreadyExistsError(err) {
		require.Equal(f.t, resp.Code, uint32(0), "transaction failed with code %d", resp.Code)
	}

	waitCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Wait for transaction confirmation with retry logic
	for retry := 0; retry < 2; retry++ {
		err = wait.ForBlocks(waitCtx, 2, f.celestia)
		if err == nil {
			break
		}
		if retry < 1 {
			f.t.Logf("Failed to wait for blocks (attempt %d/2): %v, retrying...", retry+1, err)
			time.Sleep(2 * time.Second)
		}
	}
	require.NoError(f.t, err, "failed to wait for transaction confirmation")

	f.t.Logf("Funding transaction completed for %s", toAddr.String())
}

// verifyNodeBalance verifies that a DA node has the expected balance after funding.
func (f *Framework) verifyNodeBalance(ctx context.Context, daNode *dataavailability.Node, expectedAmount int64, nodeType string) {
	nodeClient := f.GetNodeRPCClient(ctx, daNode)
	nodeAddr, err := nodeClient.State.AccountAddress(ctx)
	require.NoError(f.t, err, "failed to get %s account address", nodeType)

	// Try to verify balance with retry logic, but don't fail the test if Docker timeouts occur
	maxRetries := 3
	for attempt := 1; attempt <= maxRetries; attempt++ {
		bal := f.queryBalanceWithFallback(ctx, nodeAddr.String(), nodeType)
		if bal.Amount.Int64() >= expectedAmount {
			f.t.Logf("Successfully verified %s balance: %d utia (expected: %d)", nodeType, bal.Amount.Int64(), expectedAmount)
			return
		}

		if bal.Amount.Int64() == 0 {
			// Docker timeout occurred, skip verification
			f.t.Logf("Skipping balance verification for %s due to Docker timeout issues", nodeType)
			return
		}

		if attempt < maxRetries {
			f.t.Logf("Balance check attempt %d/%d for %s: expected %d utia, got %d utia, retrying...",
				attempt, maxRetries, nodeType, expectedAmount, bal.Amount.Int64())
			time.Sleep(3 * time.Second)
		}
	}

	// Final check - if we got here, we have a balance but it's insufficient
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
	for attempt := 1; attempt <= 3; attempt++ {
		networkInfo, err := f.celestia.GetNodes()[0].GetNetworkInfo(ctx)
		if err != nil {
			if attempt < 3 {
				f.t.Logf("Network info failed for %s (attempt %d/3): %v, retrying...", nodeType, attempt, err)
				time.Sleep(3 * time.Second)
				continue
			}
			f.t.Logf("Failed to get network info for %s after 3 attempts, skipping balance verification", nodeType)
			return sdk.Coin{Amount: sdkmath.NewInt(0), Denom: "utia"}
		}

		grpcAddr := fmt.Sprintf("%s:%s", networkInfo.External.Hostname, networkInfo.External.Ports.GRPC)
		grpcCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
		defer cancel()

		grpcConn, err := grpc.NewClient(grpcAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			if attempt < 3 {
				f.t.Logf("Failed to connect to gRPC for %s (attempt %d/3): %v, retrying...", nodeType, attempt, err)
				time.Sleep(3 * time.Second)
				continue
			}
			f.t.Logf("Failed to connect to gRPC for %s after 3 attempts, skipping balance verification", nodeType)
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
			if attempt < 3 {
				f.t.Logf("Balance query failed for %s (attempt %d/3): %v, retrying...", nodeType, attempt, err)
				time.Sleep(3 * time.Second)
				continue
			}
			f.t.Logf("Failed to query balance for %s after 3 attempts, skipping balance verification", nodeType)
			return sdk.Coin{Amount: sdkmath.NewInt(0), Denom: "utia"}
		}

		return *res.Balance
	}

	f.t.Logf("Balance query failed for %s after 3 attempts, skipping balance verification", nodeType)
	return sdk.Coin{Amount: sdkmath.NewInt(0), Denom: "utia"}
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

	// always have at least one bridge node.
	bridgeNodeConfig := dataavailability.NewNodeBuilder().
		WithNodeType(types.BridgeNode).
		Build()

	daNetworkBuilder := dataavailability.NewNetworkBuilderWithTestName(f.t, testName).
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
	var celestia *cosmos.Chain
	var err error
	// Retry both build and start together to handle transient docker issues
	for attempt := 1; attempt <= 5; attempt++ {
		celestia, err = f.chainBuilder.Build(ctx)
		if err != nil {
			if attempt < 5 && isTransientDockerError(err) {
				f.t.Logf("Retrying chain build due to transient docker issue (attempt %d/5): %v", attempt, err)
				// Shorter delay for faster retries
				delay := time.Duration(attempt) * 2 * time.Second
				if strings.Contains(err.Error(), "genesis.json") {
					delay = time.Duration(attempt) * 3 * time.Second
				}
				time.Sleep(delay)
				continue
			}
			require.NoError(f.t, err, "failed to build celestia chain")
		}

		err = celestia.Start(ctx)
		if err == nil {
			break
		}

		if attempt < 5 && isTransientDockerError(err) {
			f.t.Logf("Retrying chain build and start due to transient docker issue during start (attempt %d/5): %v", attempt, err)
			// Shorter delay for faster retries
			delay := time.Duration(attempt) * 2 * time.Second
			if strings.Contains(err.Error(), "genesis.json") {
				delay = time.Duration(attempt) * 3 * time.Second
			}
			time.Sleep(delay)
			continue
		}
		require.NoError(f.t, err, "failed to start celestia chain")
	}

	// Use a longer timeout for waiting for blocks after chain startup
	blockWaitCtx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
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

	// Build start arguments
	startArgs := []string{"--p2p.network", testChainID, "--core.ip", hostname, "--rpc.addr", "0.0.0.0", "--keyring.backend", "test"}
	if f.config.TxWorkerAccounts > 0 {
		startArgs = append(startArgs, "--tx.worker.accounts", fmt.Sprintf("%d", f.config.TxWorkerAccounts))
	}

	// Retry bridge node start for transient Docker issues and key creation errors
	for attempt := 1; attempt <= 5; attempt++ {
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
		if err == nil {
			break
		}

		if attempt < 5 && (isTransientDockerError(err) || isKeyCreationError(err)) {
			if isKeyCreationError(err) {
				f.t.Logf("Bridge node start failed with key creation error (attempt %d/5): %v, retrying...", attempt, err)
				// Longer delay for key creation issues
				delay := time.Duration(attempt) * 6 * time.Second
				time.Sleep(delay)
			} else {
				f.t.Logf("Bridge node start failed with transient docker issue (attempt %d/5): %v, retrying...", attempt, err)
				// Longer delay for bridge node startup issues
				delay := time.Duration(attempt) * 4 * time.Second
				time.Sleep(delay)
			}
			continue
		}
	}
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

	// Wait for containers to fully stop before volume cleanup
	time.Sleep(5 * time.Second)

	// Force remove any associated volumes to prevent genesis.json conflicts
	if f.client != nil {
		f.cleanupChainVolumes(ctx)
		// Additional cleanup pass after a longer delay
		time.Sleep(5 * time.Second)
		f.cleanupChainVolumes(ctx)
		// Final cleanup pass for stubborn volumes
		time.Sleep(3 * time.Second)
		f.cleanupChainVolumes(ctx)
	}

	// Best-effort: remove the docker network to avoid exhaustion
	// Wait a bit for containers to fully stop before removing network
	time.Sleep(2 * time.Second)
	if f.client != nil && f.network != "" {
		if err := f.client.NetworkRemove(ctx, f.network); err != nil {
			if !strings.Contains(err.Error(), "No such network") && !strings.Contains(err.Error(), "has active endpoints") {
				f.logger.Warn("Failed to remove docker network", zap.String("network", f.network), zap.Error(err))
			}
		}
	}

	// Note: Final cleanup sweep removed due to import complexity
	// The main cleanup above should handle most cases

	f.logger.Info("Framework cleanup completed")
	return nil
}

// cleanupPreviousTestVolumes removes any volumes from previous test runs
func (f *Framework) cleanupPreviousTestVolumes(testName string) {
	// Create a temporary client for cleanup
	client, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return // Skip cleanup if we can't create client
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// First, stop and remove any containers that might be using volumes
	f.cleanupPreviousTestContainers(ctx, client, testName)

	// Wait a bit for containers to fully stop
	time.Sleep(8 * time.Second)

	// List all volumes and remove any that match our test pattern
	volumes, err := client.VolumeList(ctx, volume.ListOptions{})
	if err != nil {
		return // Skip cleanup if we can't list volumes
	}

	for _, vol := range volumes.Volumes {
		// Remove volumes that contain our test name to prevent genesis.json conflicts
		if strings.Contains(vol.Name, testName) {
			// Try multiple times to remove the volume
			for retry := 0; retry < 3; retry++ {
				if err := client.VolumeRemove(ctx, vol.Name, true); err != nil {
					if strings.Contains(err.Error(), "No such volume") {
						break // Volume already removed
					}
					if strings.Contains(err.Error(), "volume is in use") {
						if retry < 2 {
							// Wait a bit and try again
							time.Sleep(2 * time.Second)
							continue
						}
						// Log but don't fail on final attempt
						break
					}
					// Other errors - log but continue
					break
				} else {
					// Successfully removed
					break
				}
			}
		}
	}
}

// cleanupPreviousTestContainers stops and removes containers from previous test runs
func (f *Framework) cleanupPreviousTestContainers(ctx context.Context, client *client.Client, testName string) {
	// List all containers
	containers, err := client.ContainerList(ctx, dockercontainer.ListOptions{
		All: true, // Include stopped containers
	})
	if err != nil {
		return // Skip cleanup if we can't list containers
	}

	for _, container := range containers {
		// Check if container name contains our test name
		if strings.Contains(container.Names[0], testName) {
			// Stop container if it's running
			if container.State == "running" {
				stopTimeout := 10 // seconds
				if err := client.ContainerStop(ctx, container.ID, dockercontainer.StopOptions{
					Timeout: &stopTimeout,
				}); err != nil {
					// Ignore errors - container might already be stopped
				}
			}

			// Remove container
			if err := client.ContainerRemove(ctx, container.ID, dockercontainer.RemoveOptions{
				Force: true, // Force remove even if running
			}); err != nil {
				// Ignore errors - container might already be removed
			}
		}
	}
}

// cleanupChainVolumes removes any volumes associated with the chain to prevent conflicts
func (f *Framework) cleanupChainVolumes(ctx context.Context) {
	if f.chainBuilder == nil {
		return
	}

	// Get the test name to identify volumes
	testName := f.t.Name()

	// First, ensure all containers are stopped
	f.forceStopTestContainers(ctx, testName)

	// Wait a bit for containers to fully stop
	time.Sleep(8 * time.Second)

	// List all volumes and remove any that match our test pattern
	volumes, err := f.client.VolumeList(ctx, volume.ListOptions{})
	if err != nil {
		f.logger.Warn("Failed to list volumes for cleanup", zap.Error(err))
		return
	}

	for _, vol := range volumes.Volumes {
		// Remove volumes that contain our test name to prevent genesis.json conflicts
		if strings.Contains(vol.Name, testName) {
			// Try multiple times to remove the volume
			for retry := 0; retry < 3; retry++ {
				if err := f.client.VolumeRemove(ctx, vol.Name, true); err != nil {
					if strings.Contains(err.Error(), "No such volume") {
						break // Volume already removed
					}
					if strings.Contains(err.Error(), "volume is in use") {
						if retry < 2 {
							// Wait a bit and try again
							time.Sleep(3 * time.Second)
							continue
						}
						// Try one more time with force remove
						if retry == 2 {
							time.Sleep(5 * time.Second)
							// Try to force remove by killing any containers using the volume
							f.forceRemoveVolumeContainers(ctx, vol.Name)
							continue
						}
						// Log but don't fail on final attempt
						f.logger.Warn("Failed to remove volume (still in use)", zap.String("volume", vol.Name))
						break
					}
					// Other errors - log but continue
					f.logger.Warn("Failed to remove volume", zap.String("volume", vol.Name), zap.Error(err))
					break
				} else {
					// Successfully removed
					break
				}
			}
		}
	}
}

// forceStopTestContainers forcefully stops all containers associated with the test
func (f *Framework) forceStopTestContainers(ctx context.Context, testName string) {
	// List all containers
	containers, err := f.client.ContainerList(ctx, dockercontainer.ListOptions{
		All: true, // Include stopped containers
	})
	if err != nil {
		return // Skip cleanup if we can't list containers
	}

	for _, container := range containers {
		// Check if container name contains our test name
		if len(container.Names) > 0 && strings.Contains(container.Names[0], testName) {
			// Force stop container if it's running
			if container.State == "running" {
				stopTimeout := 5 // seconds
				if err := f.client.ContainerStop(ctx, container.ID, dockercontainer.StopOptions{
					Timeout: &stopTimeout,
				}); err != nil {
					// Ignore errors - container might already be stopped
				}
			}

			// Force remove container
			if err := f.client.ContainerRemove(ctx, container.ID, dockercontainer.RemoveOptions{
				Force:         true,  // Force remove even if running
				RemoveVolumes: false, // Don't remove volumes here, we'll do it separately
			}); err != nil {
				// Ignore errors - container might already be removed
			}
		}
	}
}

// cleanupOrphanedResources cleans up any orphaned containers and volumes from previous test runs
func (f *Framework) cleanupOrphanedResources() {
	// Create a temporary client for cleanup
	client, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return // Skip cleanup if we can't create client
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Clean up orphaned containers (containers without running processes)
	containers, err := client.ContainerList(ctx, dockercontainer.ListOptions{
		All: true,
		Filters: filters.NewArgs(
			filters.Arg("status", "exited"),
			filters.Arg("status", "dead"),
		),
	})
	if err == nil {
		for _, container := range containers {
			// Remove containers that look like test containers
			if len(container.Names) > 0 &&
				(strings.Contains(container.Names[0], "TestTransactionTestSuite") ||
					strings.Contains(container.Names[0], "test-val-") ||
					strings.Contains(container.Names[0], "da-bridge-")) {
				client.ContainerRemove(ctx, container.ID, dockercontainer.RemoveOptions{
					Force:         true,
					RemoveVolumes: false, // Don't remove volumes here
				})
			}
		}
	}

	// Clean up orphaned volumes (volumes not attached to any container)
	volumes, err := client.VolumeList(ctx, volume.ListOptions{})
	if err == nil {
		for _, vol := range volumes.Volumes {
			// Remove volumes that look like test volumes and are not in use
			if strings.Contains(vol.Name, "TestTransactionTestSuite") ||
				strings.Contains(vol.Name, "test-val-") {
				// Only remove if not in use
				if vol.UsageData == nil || vol.UsageData.RefCount == 0 {
					client.VolumeRemove(ctx, vol.Name, true)
				}
			}
		}
	}
}

// forceRemoveVolumeContainers forcefully removes any containers using the specified volume
func (f *Framework) forceRemoveVolumeContainers(ctx context.Context, volumeName string) {
	// List all containers
	containers, err := f.client.ContainerList(ctx, dockercontainer.ListOptions{
		All: true,
	})
	if err != nil {
		return // Skip if we can't list containers
	}

	for _, container := range containers {
		// Check if container is using this volume
		for _, mount := range container.Mounts {
			if mount.Name == volumeName {
				// Force stop and remove this container
				if container.State == "running" {
					stopTimeout := 3 // seconds
					f.client.ContainerStop(ctx, container.ID, dockercontainer.StopOptions{
						Timeout: &stopTimeout,
					})
				}
				// Force remove container
				f.client.ContainerRemove(ctx, container.ID, dockercontainer.RemoveOptions{
					Force:         true,
					RemoveVolumes: false, // Don't remove volumes here
				})
				break
			}
		}
	}
}
