package nodebuilder

import (
	"context"
	"fmt"

	"go.uber.org/fx"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/libs/fxutil"
	"github.com/celestiaorg/celestia-node/nodebuilder/blob"
	"github.com/celestiaorg/celestia-node/nodebuilder/blobstream"
	"github.com/celestiaorg/celestia-node/nodebuilder/core"
	"github.com/celestiaorg/celestia-node/nodebuilder/da"
	"github.com/celestiaorg/celestia-node/nodebuilder/das"
	"github.com/celestiaorg/celestia-node/nodebuilder/fraud"
	modhead "github.com/celestiaorg/celestia-node/nodebuilder/header"
	"github.com/celestiaorg/celestia-node/nodebuilder/node"
	"github.com/celestiaorg/celestia-node/nodebuilder/p2p"
	"github.com/celestiaorg/celestia-node/nodebuilder/pruner"
	"github.com/celestiaorg/celestia-node/nodebuilder/rpc"
	"github.com/celestiaorg/celestia-node/nodebuilder/share"
	"github.com/celestiaorg/celestia-node/nodebuilder/state"
)

func ConstructModule(tp node.Type, network p2p.Network, cfg *Config, store Store) fx.Option {
	log.Infow("Accessing keyring...")
	ks, err := store.Keystore()
	if err != nil {
		return fx.Error(err)
	}

	// Validate P2P config with Share config (for cross-module validation)
	if err := cfg.P2P.ValidateWithShareConfig(tp, &cfg.Share); err != nil {
		return fx.Error(fmt.Errorf("nodebuilder: config validation: %w", err))
	}

	baseComponents := fx.Options(
		fx.Supply(tp),
		fx.Supply(network),
		fx.Supply(ks),
		fx.Provide(p2p.BootstrappersFor),
		fx.Provide(func(lc fx.Lifecycle) context.Context {
			return fxutil.WithLifecycle(context.Background(), lc)
		}),
		fx.Supply(cfg),
		fx.Supply(store.Config),
		fx.Provide(store.Datastore),
		fx.Provide(store.Keystore),
		core.ConstructModule(tp, &cfg.Core, &cfg.Share),
		fx.Supply(node.StorePath(store.Path())),
		// modules provided by the node
		p2p.ConstructModule(tp, &cfg.P2P),
		modhead.ConstructModule[*header.ExtendedHeader](tp, &cfg.Header, &cfg.P2P),
		share.ConstructModule(tp, &cfg.Share, &cfg.P2P),
		state.ConstructModule(tp, &cfg.State, &cfg.Core),
		das.ConstructModule(tp, &cfg.DASer),
		fraud.ConstructModule(tp),
		blob.ConstructModule(),
		da.ConstructModule(),
		node.ConstructModule(tp),
		pruner.ConstructModule(tp),
		rpc.ConstructModule(tp, &cfg.RPC),
		blobstream.ConstructModule(),
	)

	return fx.Module(
		"node",
		baseComponents,
	)
}
