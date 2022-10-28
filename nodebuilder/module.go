package nodebuilder

import (
	"context"

	"go.uber.org/fx"

	"github.com/celestiaorg/celestia-node/libs/fxutil"
	"github.com/celestiaorg/celestia-node/nodebuilder/core"
	"github.com/celestiaorg/celestia-node/nodebuilder/das"
	"github.com/celestiaorg/celestia-node/nodebuilder/fraud"
	"github.com/celestiaorg/celestia-node/nodebuilder/header"
	"github.com/celestiaorg/celestia-node/nodebuilder/node"
	"github.com/celestiaorg/celestia-node/nodebuilder/p2p"
	"github.com/celestiaorg/celestia-node/nodebuilder/rpc"
	"github.com/celestiaorg/celestia-node/nodebuilder/share"
	"github.com/celestiaorg/celestia-node/nodebuilder/state"
)

func ConstructModule(tp node.Type, network p2p.Network, cfg *Config, store Store) fx.Option {
	baseComponents := fx.Options(
		fx.Supply(tp),
		fx.Supply(network),
		fx.Provide(p2p.BootstrappersFor),
		fx.Provide(func(lc fx.Lifecycle) context.Context {
			return fxutil.WithLifecycle(context.Background(), lc)
		}),
		fx.Supply(cfg),
		fx.Supply(store.Config),
		fx.Provide(store.Datastore),
		fx.Provide(store.Keystore),
		// modules provided by the node
		p2p.ConstructModule(tp, &cfg.P2P),
		state.ConstructModule(tp, &cfg.State),
		header.ConstructModule(tp, &cfg.Header),
		share.ConstructModule(tp, &cfg.Share),
		rpc.ConstructModule(tp, &cfg.RPC),
		core.ConstructModule(tp, &cfg.Core),
		das.ConstructModule(tp, &cfg.DASer),
		fraud.ConstructModule(tp),
	)

	return fx.Module(
		"node",
		baseComponents,
	)
}
