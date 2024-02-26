package rpc

import (
	"context"

	"go.uber.org/fx"

	"github.com/celestiaorg/celestia-node/api/rpc"
	"github.com/celestiaorg/celestia-node/nodebuilder/node"
)

func ConstructModule(tp node.Type, cfg *Config) fx.Option {
	// sanitize config values before constructing module
	cfgErr := cfg.Validate()

	baseComponents := fx.Options(
		fx.Supply(cfg),
		fx.Error(cfgErr),
		fx.Provide(fx.Annotate(
			server,
			fx.OnStart(func(ctx context.Context, server *rpc.Server) error {
				return server.Start(ctx)
			}),
			fx.OnStop(func(ctx context.Context, server *rpc.Server) error {
				return server.Stop(ctx)
			}),
		)),
	)

	switch tp {
	case node.Light, node.Full, node.Bridge:
		return fx.Module(
			"rpc",
			baseComponents,
			fx.Invoke(registerEndpoints),
		)
	default:
		panic("invalid node type")
	}
}
