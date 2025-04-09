package core

import (
	"context"
	sdklog "cosmossdk.io/log"
	"path/filepath"
	"testing"
	"time"

	"github.com/celestiaorg/celestia-app/v4/test/util/genesis"
	"github.com/celestiaorg/celestia-app/v4/test/util/testnode"
	tmrand "github.com/cometbft/cometbft/libs/rand"
	"github.com/cometbft/cometbft/node"
	cmthttp "github.com/cometbft/cometbft/rpc/client/http"
	srvtypes "github.com/cosmos/cosmos-sdk/server/types"
	"github.com/stretchr/testify/require"
)

const chainID = "private"

// DefaultTestConfig returns the default testing configuration for Tendermint + Celestia App tandem.
//
// It fetches free ports from OS and sets them into configs, s.t.
// user can make use of them(unlike 0 port) and allowing to run
// multiple tests nodes in parallel.
//
// Additionally, it instructs Tendermint + Celestia App tandem to setup 10 funded accounts.
func DefaultTestConfig() *testnode.Config {
	genesis := genesis.NewDefaultGenesis().
		WithChainID(chainID).
		WithValidators(genesis.NewDefaultValidator(testnode.DefaultValidatorAccountName)).
		WithConsensusParams(testnode.DefaultConsensusParams())

	tmConfig := testnode.DefaultTendermintConfig()
	tmConfig.Consensus.TimeoutCommit = time.Millisecond * 200

	return testnode.DefaultConfig().
		WithGenesis(genesis).
		WithFundedAccounts(generateRandomAccounts(10)...). // 10 usually is enough for testing
		WithChainID(chainID).
		WithTendermintConfig(tmConfig).
		WithSuppressLogs(true)
}

// StartTestNode simply starts Tendermint and Celestia App tandem with default testing
// configuration.
func StartTestNode(t *testing.T) testnode.Context {
	return StartTestNodeWithConfig(t, DefaultTestConfig())
}

// StartTestNodeWithConfig starts Tendermint and Celestia App tandem with custom configuration.
func StartTestNodeWithConfig(t *testing.T, cfg *testnode.Config) testnode.Context {
	cctx, _, _ := testnode.NewNetwork(t, cfg)
	// we want to test over remote http client,
	// so we are as close to the real environment as possible,
	// however, it might be useful to use a local tendermint client
	// if you need to debug something inside it
	return cctx
}

// StartTestNodeWithConfigAndClient initializes a test node with default configuration and a WebSocket HTTP client.
func StartTestNodeWithConfigAndClient(t *testing.T) (testnode.Context, *cmthttp.HTTP) {
	cctx, rpcAddr, _ := testnode.NewNetwork(t, DefaultTestConfig())
	wsClient, err := cmthttp.New(rpcAddr, "/websocket")
	if err != nil {
		panic(err)
	}
	return cctx, wsClient
}

// generateRandomAccounts generates n random account names.
func generateRandomAccounts(n int) []string {
	accounts := make([]string, n)
	for i := range accounts {
		accounts[i] = tmrand.Str(9)
	}
	return accounts
}

// Network wraps `testnode.Context` allowing to manually stop all underlying connections.
// TODO @vgonkivs: remove after https://github.com/celestiaorg/celestia-app/issues/4304 is done.
type Network struct {
	testnode.Context
	config *testnode.Config
	app    srvtypes.Application
	tmNode *node.Node

	stopNode func() error
	stopGRPC func() error
	stopAPI  func() error
}

func NewNetwork(t testing.TB, config *testnode.Config) *Network {
	t.Helper()

	// initialize the genesis file and validator files for the first validator.
	baseDir := filepath.Join(t.TempDir(), "testnode")
	err := genesis.InitFiles(baseDir, config.TmConfig, config.AppConfig, config.Genesis, 0)
	require.NoError(t, err)

	tmNode, app, err := testnode.NewCometNode(baseDir, &config.UniversalTestingConfig)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(func() {
		cancel()
	})

	cctx := testnode.NewContext(
		ctx,
		config.Genesis.Keyring(),
		config.TmConfig,
		config.Genesis.ChainID,
		config.AppConfig.API.Address,
	)

	return &Network{
		Context: cctx,
		config:  config,
		app:     app,
		tmNode:  tmNode,
	}
}

func (n *Network) Start() error {
	cctx, stopNode, err := testnode.StartNode(n.tmNode, n.Context)
	if err != nil {
		return err
	}

	// TODO: put a real logger
	grpcSrv, cctx, cleanupGRPC, err := testnode.StartGRPCServer(sdklog.NewNopLogger(), n.app, n.config.AppConfig, cctx)
	if err != nil {
		return err
	}

	apiServer, err := testnode.StartAPIServer(n.app, *n.config.AppConfig, cctx, grpcSrv)
	if err != nil {
		return err
	}

	n.Context = cctx
	n.stopNode = stopNode
	n.stopGRPC = cleanupGRPC
	n.stopAPI = apiServer.Close
	return nil
}

func (n *Network) Stop() error {
	err := n.stopNode()
	if err != nil {
		return err
	}

	err = n.stopGRPC()
	if err != nil {
		return err
	}

	err = n.stopAPI()
	if err != nil {
		return err
	}
	return nil
}
