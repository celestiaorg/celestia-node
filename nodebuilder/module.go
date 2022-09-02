package nodebuilder

import (
	"context"

	"go.uber.org/fx"

	"github.com/celestiaorg/celestia-node/nodebuilder/core"
	"github.com/celestiaorg/celestia-node/nodebuilder/daser"
	"github.com/celestiaorg/celestia-node/nodebuilder/header"
	"github.com/celestiaorg/celestia-node/nodebuilder/node"
	"github.com/celestiaorg/celestia-node/nodebuilder/p2p"
	"github.com/celestiaorg/celestia-node/nodebuilder/rpc"
	"github.com/celestiaorg/celestia-node/nodebuilder/share"
	"github.com/celestiaorg/celestia-node/nodebuilder/state"
	"github.com/celestiaorg/celestia-node/params"
)

func Module(tp node.Type, cfg *Config, store Store) fx.Option {
	baseComponents := fx.Options(
		fx.Provide(params.DefaultNetwork),
		fx.Provide(params.BootstrappersFor),
		fx.Provide(context.Background),
		fx.Supply(cfg),
		fx.Supply(store.Config),
		fx.Provide(store.Datastore),
		fx.Provide(store.Keystore),
		fx.Invoke(invokeWatchdog(store.Path())),
		// modules provided by the node
		p2p.Module(&cfg.P2P),
		state.Module(tp, &cfg.State),
		header.Module(tp, &cfg.Header),
		share.Module(tp, &cfg.Share),
		rpc.Module(tp, &cfg.RPC),
		core.Module(tp, &cfg.Core),
		daser.Module(tp),
	)

	return fx.Module(
		"node",
		fx.Supply(tp),
		baseComponents,
	)
}
