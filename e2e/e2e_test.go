package e2e

import (
	"context"
	"github.com/celestiaorg/celestia-app/v4/app"
	celestiadockertypes "github.com/celestiaorg/tastora/framework/docker"
	"github.com/celestiaorg/tastora/framework/testutil/toml"
	celestiatypes "github.com/celestiaorg/tastora/framework/types"
	"github.com/cosmos/cosmos-sdk/types/module/testutil"
	"github.com/moby/moby/client"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
	"os"
	"testing"
)

const (
	celestiaAppImage   = "ghcr.io/celestiaorg/celestia-app"
	defaultCelestiaTag = "v4.0.0-rc6"
	nodeImage          = "ghcr.io/celestiaorg/celestia-node"
	defaultNodeTag     = "v0.23.0-rc0"
	testChainID        = "test"
)

func TestCelestiaTestSuite(t *testing.T) {
	suite.Run(t, new(CelestiaTestSuite))
}

type CelestiaTestSuite struct {
	suite.Suite
	logger  *zap.Logger
	client  *client.Client
	network string
}

func (s *CelestiaTestSuite) SetupSuite() {
	s.logger = zaptest.NewLogger(s.T())
	s.logger.Info("Setting up Celestia test suite")
	s.client, s.network = celestiadockertypes.DockerSetup(s.T())
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

func (s *CelestiaTestSuite) CreateDockerProvider() celestiatypes.Provider {
	numValidators := 1
	numFullNodes := 0

	enc := testutil.MakeTestEncodingConfig(app.ModuleEncodingRegisters...)

	cfg := celestiadockertypes.Config{
		Logger:          s.logger,
		DockerClient:    s.client,
		DockerNetworkID: s.network,
		ChainConfig: &celestiadockertypes.ChainConfig{
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
			Images: []celestiadockertypes.DockerImage{
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
		DANodeConfig: &celestiadockertypes.DANodeConfig{
			ChainID: testChainID,
			Images: []celestiadockertypes.DockerImage{
				{
					Repository: getNodeImage(),
					Version:    getNodeTag(),
					UIDGID:     "10001:10001",
				},
			}},
	}
	return celestiadockertypes.NewProvider(cfg, s.T())
}

// getGenesisHash returns the genesis hash of the given chain node.
func (s *CelestiaTestSuite) getGenesisHash(ctx context.Context, node celestiatypes.ChainNode) string {
	c, err := node.GetRPCClient()
	s.Require().NoError(err, "failed to get node client")

	first := int64(1)
	block, err := c.Block(ctx, &first)
	s.Require().NoError(err, "failed to get block")

	return block.Block.Header.Hash().String()
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
