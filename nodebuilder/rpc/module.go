package rpc

import (
	"context"

	"go.uber.org/fx"

	headerServ "github.com/celestiaorg/celestia-node/nodebuilder/header"
	"github.com/celestiaorg/celestia-node/nodebuilder/node"
	stateServ "github.com/celestiaorg/celestia-node/nodebuilder/state"
	rpcServ "github.com/celestiaorg/celestia-node/service/rpc"
	shareServ "github.com/celestiaorg/celestia-node/service/share"
)

func Module(tp node.Type, cfg *rpcServ.Config) fx.Option {
	// sanitize config values before constructing module
	cfgErr := cfg.Validate()

	baseComponents := fx.Options(
		fx.Supply(*cfg),
		fx.Error(cfgErr),
		fx.Provide(fx.Annotate(
			rpcServ.NewServer,
			fx.OnStart(func(ctx context.Context, server *rpcServ.Server) error {
				return server.Start(ctx)
			}),
			fx.OnStop(func(ctx context.Context, server *rpcServ.Server) error {
				return server.Stop(ctx)
			}),
		)),
	)

	switch tp {
	case node.Light, node.Full:
		return fx.Module(
			"rpc",
			baseComponents,
			fx.Invoke(Handler),
		)
	case node.Bridge:
		return fx.Module(
			"rpc",
			baseComponents,
			fx.Invoke(func(
				state stateServ.Service,
				share *shareServ.Service,
				header headerServ.Service,
				rpcSrv *rpcServ.Server,
			) {
				Handler(state, share, header, rpcSrv, nil)
			}),
		)
	default:
		panic("invalid node type")
	}
}
