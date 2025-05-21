package e2e

import (
	"context"
	sdkmath "cosmossdk.io/math"
	rpcclient "github.com/celestiaorg/celestia-node/api/rpc/client"
	"github.com/celestiaorg/tastora/framework/testutil/sdkacc"
	"github.com/celestiaorg/tastora/framework/testutil/wait"
	sdk "github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"go.uber.org/zap/zaptest"
	"google.golang.org/grpc"
	"os"
	"testing"

	"github.com/celestiaorg/celestia-app/v4/app"
	tastoradockertypes "github.com/celestiaorg/tastora/framework/docker"
	"github.com/celestiaorg/tastora/framework/testutil/toml"
	tastoratypes "github.com/celestiaorg/tastora/framework/types"
	"github.com/cosmos/cosmos-sdk/types/module/testutil"
	"github.com/moby/moby/client"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"
)

const (
	celestiaAppImage   = "ghcr.io/celestiaorg/celestia-app"
	defaultCelestiaTag = "v4.0.0-rc6"
	nodeImage          = "celestia-node"
	defaultNodeTag     = "foo"
	testChainID        = "test"
)

func TestCelestiaTestSuite(t *testing.T) {
	suite.Run(t, new(CelestiaTestSuite))
}

type CelestiaTestSuite struct {
	suite.Suite
	logger     *zap.Logger
	client     *client.Client
	network    string
	provider   tastoratypes.Provider
	bridgeNode tastoratypes.DANode
	fullNode   tastoratypes.DANode
	lightNode  tastoratypes.DANode
	celestia   tastoratypes.Chain
}

func (s *CelestiaTestSuite) SetupTest() {
	s.logger = zaptest.NewLogger(s.T())
	s.logger.Info("Setting up test", zap.String("test", s.T().Name()))
	s.client, s.network = tastoradockertypes.DockerSetup(s.T())
	s.provider = s.CreateDockerProvider()

	ctx := context.Background()
	s.celestia = s.CreateAndStartCelestiaChain(ctx)
	s.bridgeNode = s.CreateAndStartBridgeNode(ctx, s.celestia)
	s.fullNode = s.CreateAndStartFullNode(ctx, s.bridgeNode, s.celestia)
	s.lightNode = s.CreateAndStartLightNode(ctx, s.fullNode, s.celestia)
}

// appOverrides modifies the "app.toml" configuration for the application, setting the transaction indexer to "kv".
func appOverrides() toml.Toml {
	// required to query tx by hash when broadcasting transactions.
	appTomlOverride := make(toml.Toml)
	txIndexConfig := make(toml.Toml)
	txIndexConfig["indexer"] = "kv"
	appTomlOverride["tx-index"] = txIndexConfig
	return appTomlOverride
}

// configOverrides modifies the "config.toml" configuration, enabling KV indexing for transaction queries.
func configOverrides() toml.Toml {
	// required to query tx by hash when broadcasting transactions.
	overrides := make(toml.Toml)
	txIndexConfig := make(toml.Toml)
	txIndexConfig["indexer"] = "kv"
	overrides["tx_index"] = txIndexConfig
	return overrides
}

func (s *CelestiaTestSuite) CreateDockerProvider() tastoratypes.Provider {
	numValidators := 1
	numFullNodes := 0

	enc := testutil.MakeTestEncodingConfig(app.ModuleEncodingRegisters...)

	cfg := tastoradockertypes.Config{
		Logger:          s.logger,
		DockerClient:    s.client,
		DockerNetworkID: s.network,
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
			Bin:                 "celestia-appd",
			Bech32Prefix:        "celestia",
			Denom:               "utia",
			CoinType:            "118",
			GasPrices:           "0.025utia",
			GasAdjustment:       1.3,
			EncodingConfig:      &enc,
			AdditionalStartArgs: []string{"--force-no-bbr", "--grpc.enable", "--grpc.address", "0.0.0.0:9090", "--rpc.grpc_laddr=tcp://0.0.0.0:9098"},
		},
		DANodeConfig: &tastoradockertypes.DANodeConfig{
			ChainID: testChainID,
			Images: []tastoradockertypes.DockerImage{
				{
					Repository: getNodeImage(),
					Version:    getNodeTag(),
					UIDGID:     "10001:10001",
				},
			}},
	}
	return tastoradockertypes.NewProvider(cfg, s.T())
}

// CreateTestWallet creates a new test wallet on the given chain, funding it with the specified amount.
func (s *CelestiaTestSuite) CreateTestWallet(ctx context.Context, celestia tastoratypes.Chain, amount int64) tastoratypes.Wallet {
	dockerChain, ok := celestia.(*tastoradockertypes.Chain)
	s.Require().True(ok, "celestia is not a docker chain")

	wallet, err := tastoradockertypes.CreateAndFundTestWallet(s.T(), ctx, "test", sdkmath.NewInt(amount), dockerChain)
	s.Require().NoError(err, "failed to create test wallet")
	s.Require().NotNil(wallet, "wallet is nil")
	return wallet
}

// CreateAndStartCelestiaChain initializes and starts a a Celestia chain, ensuring it successfully begins producing blocks.
func (s *CelestiaTestSuite) CreateAndStartCelestiaChain(ctx context.Context) tastoratypes.Chain {
	celestia, err := s.provider.GetChain(ctx)
	s.Require().NoError(err, "failed to get chain")
	err = celestia.Start(ctx)
	s.Require().NoError(err)

	// verify the chain is producing blocks
	height, err := celestia.Height(ctx)
	s.Require().NoError(err)
	s.Require().Greater(height, int64(0))
	return celestia
}

// CreateAndStartBridgeNode initializes and starts a bridge node, setting it up with the required genesis hash and core IP.
func (s *CelestiaTestSuite) CreateAndStartBridgeNode(ctx context.Context, chain tastoratypes.Chain) tastoratypes.DANode {
	genesisHash := s.getGenesisHash(ctx, chain)
	s.Require().NotEmpty(genesisHash, "genesis hash is empty")

	bridgeNode, err := s.provider.GetDANode(ctx, tastoratypes.BridgeNode)
	s.Require().NoError(err, "failed to get bridge node")

	hostname, err := chain.GetNodes()[0].GetInternalHostName(ctx)
	s.Require().NoError(err, "failed to get internal hostname")

	err = bridgeNode.Start(ctx,
		tastoratypes.WithCoreIP(hostname),
		tastoratypes.WithGenesisBlockHash(genesisHash),
	)
	s.Require().NoError(err, "failed to start bridge node")
	return bridgeNode
}

// CreateAndStartFullNode initializes and starts a full node, connecting it to a bridge node and ensuring proper configuration.
func (s *CelestiaTestSuite) CreateAndStartFullNode(ctx context.Context, bridgeNode tastoratypes.DANode, chain tastoratypes.Chain) tastoratypes.DANode {
	genesisHash := s.getGenesisHash(ctx, chain)

	hostname, err := chain.GetNodes()[0].GetInternalHostName(ctx)
	s.Require().NoError(err, "failed to get internal hostname")

	p2pInfo, err := bridgeNode.GetP2PInfo(ctx)
	s.Require().NoError(err, "failed to get bridge node p2p info")

	p2pAddr, err := p2pInfo.GetP2PAddress()
	s.Require().NoError(err, "failed to get bridge node p2p address")

	fullNode, err := s.provider.GetDANode(ctx, tastoratypes.FullNode)
	s.Require().NoError(err, "failed to get fullnode node")

	err = fullNode.Start(ctx,
		tastoratypes.WithCoreIP(hostname),
		tastoratypes.WithGenesisBlockHash(genesisHash),
		tastoratypes.WithP2PAddress(p2pAddr),
	)

	s.Require().NoError(err, "failed to start full node")

	return fullNode
}

// CreateAndStartLightNode initializes and starts a light node, configuring it with the provided full node and chain details.
func (s *CelestiaTestSuite) CreateAndStartLightNode(ctx context.Context, fullNode tastoratypes.DANode, chain tastoratypes.Chain) tastoratypes.DANode {
	genesisHash := s.getGenesisHash(ctx, chain)

	hostname, err := chain.GetNodes()[0].GetInternalHostName(ctx)
	s.Require().NoError(err, "failed to get internal hostname")

	p2pInfo, err := fullNode.GetP2PInfo(ctx)
	s.Require().NoError(err, "failed to get bridge node p2p info")

	p2pAddr, err := p2pInfo.GetP2PAddress()
	s.Require().NoError(err, "failed to get bridge node p2p address")

	s.T().Logf("Full node P2P Addr: %s", p2pAddr)

	lightNode, err := s.provider.GetDANode(ctx, tastoratypes.LightNode)
	s.Require().NoError(err, "failed to get light node")

	err = lightNode.Start(ctx,
		tastoratypes.WithP2PAddress(p2pAddr),
		tastoratypes.WithCoreIP(hostname), // TODO: remove this, not required
		tastoratypes.WithGenesisBlockHash(genesisHash),
	)
	s.Require().NoError(err, "failed to start light node")

	return lightNode
}

// getGenesisHash returns the genesis hash of the given chain node.
func (s *CelestiaTestSuite) getGenesisHash(ctx context.Context, chain tastoratypes.Chain) string {
	node := chain.GetNodes()[0]
	c, err := node.GetRPCClient()
	s.Require().NoError(err, "failed to get node client")

	first := int64(1)
	block, err := c.Block(ctx, &first)
	s.Require().NoError(err, "failed to get block")

	genesisHash := block.Block.Header.Hash().String()
	s.Require().NotEmpty(genesisHash, "genesis hash is empty")
	return genesisHash
}

// GetNodeRPCClient retrieves an RPC client for the provided DA node using its host RPC address.
func (s *CelestiaTestSuite) GetNodeRPCClient(ctx context.Context, daNode tastoratypes.DANode) *rpcclient.Client {
	rpcAddr := daNode.GetHostRPCAddress()
	s.Require().NotEmpty(rpcAddr, "rpc address is empty")

	rpcClient, err := rpcclient.NewClient(ctx, "http://"+rpcAddr, "")
	s.Require().NoError(err)
	return rpcClient
}

// FundWallet sends funds from the given wallet to the given address.
// The amount is specified in utia.
func (s *CelestiaTestSuite) FundWallet(ctx context.Context, chain tastoratypes.Chain, fromWallet tastoratypes.Wallet, toAddr sdk.AccAddress, amount int64) {
	fromAddr, err := sdkacc.AddressFromBech32(fromWallet.GetFormattedAddress(), "celestia")
	s.Require().NoError(err, "failed to get from address")

	s.T().Logf("sending funds from %s to %s", fromAddr.String(), toAddr.String())

	bankSend := banktypes.NewMsgSend(fromAddr, toAddr.Bytes(), sdk.NewCoins(sdk.NewCoin("utia", sdkmath.NewInt(amount))))
	resp, err := chain.BroadcastMessages(ctx, fromWallet, bankSend)
	s.Require().NoError(err)
	s.Require().Equal(resp.Code, uint32(0), "resp: %v", resp)

	// wait for blocks to ensure the funds are available.
	s.Require().NoError(wait.ForBlocks(ctx, 2, chain))

	grpcAddr := s.celestia.GetGRPCAddress()
	bal := s.QueryBalance(ctx, grpcAddr, toAddr.String())
	// ensure the balance has at least as much as what was just sent.
	s.Require().GreaterOrEqualf(bal.Amount.Int64(), amount, "balance is not greater than or equal to %d", amount)
}

// queryBalance fetches the balance of a given address and denom from a Cosmos SDK chain via gRPC.
func (s *CelestiaTestSuite) QueryBalance(ctx context.Context, grpcAddr string, addr string) sdk.Coin {
	grpcConn, err := grpc.Dial(grpcAddr, grpc.WithInsecure())
	s.Require().NoError(err, "failed to connect to gRPC")
	bankClient := banktypes.NewQueryClient(grpcConn)
	req := &banktypes.QueryBalanceRequest{
		Address: addr,
		Denom:   "utia",
	}

	res, err := bankClient.Balance(ctx, req)
	s.Require().NoError(err, "failed to query balance")
	return *res.Balance
}

// getCelestiaTag returns the tag to use for Celestia images.
// It can be overridden by setting the CELESTIA_TAG environment.
func getCelestiaTag() string {
	if tag := os.Getenv("CELESTIA_TAG"); tag != "" {
		return tag
	}
	return defaultCelestiaTag
}

// getNodeTag returns the tag to use for Celestia Node images.
// It can be overridden by setting the CELESTIA_NODE_TAG environment.
func getNodeTag() string {
	if tag := os.Getenv("CELESTIA_NODE_TAG"); tag != "" {
		return tag
	}
	return defaultNodeTag
}

// getNodeTag returns the image to use for Celestia Nodes.
// It can be overridden by setting the CELESTIA_NODE_IMAGE environment.
func getNodeImage() string {
	if tag := os.Getenv("CELESTIA_NODE_IMAGE"); tag != "" {
		return tag
	}
	return nodeImage
}
