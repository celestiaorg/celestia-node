package nodebuilder

import (
	"context"

	"go.uber.org/fx"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/libs/fxutil"
	"github.com/celestiaorg/celestia-node/nodebuilder/blob"
	"github.com/celestiaorg/celestia-node/nodebuilder/core"
	"github.com/celestiaorg/celestia-node/nodebuilder/da"
	"github.com/celestiaorg/celestia-node/nodebuilder/das"
	"github.com/celestiaorg/celestia-node/nodebuilder/fraud"
	"github.com/celestiaorg/celestia-node/nodebuilder/gateway"
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
		fx.Supply(node.StorePath(store.Path())),
		// modules provided by the node
		p2p.ConstructModule(tp, &cfg.P2P),
		state.ConstructModule(tp, &cfg.State, &cfg.Core),
		modhead.ConstructModule[*header.ExtendedHeader](tp, &cfg.Header),
		share.ConstructModule(tp, &cfg.Share),
		gateway.ConstructModule(tp, &cfg.Gateway),
		core.ConstructModule(tp, &cfg.Core),
		das.ConstructModule(tp, &cfg.DASer),
		fraud.ConstructModule(tp),
		blob.ConstructModule(),
		da.ConstructModule(),
		node.ConstructModule(tp),
		pruner.ConstructModule(tp, &cfg.Pruner),
		rpc.ConstructModule(tp, &cfg.RPC),
	)

	return fx.Module(
		"node",
		baseComponents,
	)
}
