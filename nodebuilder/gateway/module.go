package gateway

import (
	"context"

	logging "github.com/ipfs/go-log/v2"
	"go.uber.org/fx"

	"github.com/celestiaorg/celestia-node/api/gateway"
	headerServ "github.com/celestiaorg/celestia-node/nodebuilder/header"
	"github.com/celestiaorg/celestia-node/nodebuilder/node"
	shareServ "github.com/celestiaorg/celestia-node/nodebuilder/share"
	stateServ "github.com/celestiaorg/celestia-node/nodebuilder/state"
)

var log = logging.Logger("module/gateway")

func ConstructModule(tp node.Type, cfg *Config) fx.Option {
	// sanitize config values before constructing module
	cfgErr := cfg.Validate()
	if !cfg.Enabled {
		return fx.Options()
	}
	log.Warn("Many gateway endpoints will be deprecated in the next release. Please use the RPC instead.")

	baseComponents := fx.Options(
		fx.Supply(cfg),
		fx.Error(cfgErr),
		fx.Provide(fx.Annotate(
			server,
			fx.OnStart(func(ctx context.Context, server *gateway.Server) error {
				return server.Start(ctx)
			}),
			fx.OnStop(func(ctx context.Context, server *gateway.Server) error {
				return server.Stop(ctx)
			}),
		)),
	)

	switch tp {
	case node.Light, node.Full:
		return fx.Module(
			"gateway",
			baseComponents,
			fx.Invoke(Handler),
		)
	case node.Bridge:
		return fx.Module(
			"gateway",
			baseComponents,
			fx.Invoke(func(
				cfg *Config,
				state stateServ.Module,
				share shareServ.Module,
				header headerServ.Module,
				serv *gateway.Server,
			) {
				Handler(cfg, state, share, header, nil, serv)
			}),
		)
	default:
		panic("invalid node type")
	}
}
