package core

import (
	"fmt"
	"path/filepath"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/ibc-go/testing/simapp"
	"github.com/tendermint/spm/cosmoscmd"
	corenode "github.com/tendermint/tendermint/node"
	"github.com/tendermint/tendermint/p2p"
	"github.com/tendermint/tendermint/privval"
	"github.com/tendermint/tendermint/proxy"
	"github.com/tendermint/tendermint/rpc/client"
	"github.com/tendermint/tendermint/rpc/client/http"
	"github.com/tendermint/tendermint/rpc/client/local"
	dbm "github.com/tendermint/tm-db"

	"github.com/celestiaorg/celestia-app/app"
)

// Client is an alias to Core Client.
type Client = client.Client

// NewRemote creates a new Client that communicates with a remote Core endpoint over HTTP.
func NewRemote(protocol, remoteAddr string) (Client, error) {
	return http.New(
		fmt.Sprintf("%s://%s", protocol, remoteAddr),
		"/websocket",
	)
}

// NewEmbedded returns a new Client from an embedded Core node process.
func NewEmbedded(cfg *Config) (Client, error) {
	logger := adaptedLogger()

	if cfg.ProxyApp == "kvstore" {
		node, err := corenode.DefaultNewNode(cfg, logger)
		if err != nil {
			return nil, err
		}

		return &embeddedWrapper{local.New(node), node}, nil
	}

	nodeKey, err := p2p.LoadOrGenNodeKey(cfg.NodeKeyFile())
	if err != nil {
		return nil, fmt.Errorf("failed to load or gen node key %s: %w", cfg.NodeKeyFile(), err)
	}

	db, err := openDB(cfg.RootDir)
	if err != nil {
		return nil, err
	}

	skipHeights := make(map[int64]bool)

	tiaApp := app.New(
		logger,
		db,
		nil,
		true,
		skipHeights,
		cfg.RootDir,
		0,
		cosmoscmd.MakeEncodingConfig(app.ModuleBasics),
		simapp.EmptyAppOptions{},
	)

	node, err := corenode.NewNode(
		cfg,
		privval.LoadOrGenFilePV(cfg.PrivValidatorKeyFile(), cfg.PrivValidatorStateFile()),
		nodeKey,
		proxy.NewLocalClientCreator(tiaApp),
		corenode.DefaultGenesisDocProviderFunc(cfg),
		corenode.DefaultDBProvider,
		corenode.DefaultMetricsProvider(cfg.Instrumentation),
		logger,
	)
	if err != nil {
		return nil, err
	}

	return &embeddedWrapper{local.New(node), node}, nil
}

// NewEmbeddedFromNode wraps a given Core node process to be able to control its lifecycle.
func NewEmbeddedFromNode(node *corenode.Node) Client {
	return &embeddedWrapper{local.New(node), node}
}

// embeddedWrapper is a small wrapper around local Client which ensures the embedded Core node
// can be started/stopped.
type embeddedWrapper struct {
	*local.Local
	node *corenode.Node
}

func (e *embeddedWrapper) Start() error {
	return e.node.Start()
}

func (e *embeddedWrapper) Stop() error {
	return e.node.Stop()
}

func openDB(rootDir string) (dbm.DB, error) {
	dataDir := filepath.Join(rootDir, "data")
	return sdk.NewLevelDB("application", dataDir)
}
