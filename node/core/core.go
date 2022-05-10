package core

import (
	"context"
	"fmt"

	format "github.com/ipfs/go-ipld-format"
	"go.uber.org/fx"

	"github.com/celestiaorg/celestia-node/libs/fxutil"

	"github.com/celestiaorg/celestia-node/core"
	"github.com/celestiaorg/celestia-node/header"
	coreexchange "github.com/celestiaorg/celestia-node/header/core"
)

// Config combines all configuration fields for managing the relationship with a Core node.
type Config struct {
	Protocol   string
	RemoteAddr string
	GRPCAddr   string
}

// DefaultConfig returns default configuration for Core subsystem.
func DefaultConfig() Config {
	return Config{}
}

// Components collects all the components and services related to managing the relationship with the Core node.
func Components(cfg Config) fx.Option {
	return fx.Options(
		fx.Provide(core.NewBlockFetcher),
		fxutil.ProvideAs(coreexchange.NewExchange, new(header.Exchange)),
		fx.Invoke(HeaderListener),
		fx.Provide(func(lc fx.Lifecycle) (core.Client, error) {
			if cfg.RemoteAddr == "" {
				return nil, fmt.Errorf("no celestia-core endpoint given")
			}
			client, err := RemoteClient(cfg)
			if err != nil {
				return nil, err
			}
			lc.Append(fx.Hook{
				OnStart: func(context.Context) error {
					return client.Start()
				},
				OnStop: func(context.Context) error {
					return client.Stop()
				},
			})

			return client, err
		}),
	)
}

func HeaderListener(
	lc fx.Lifecycle,
	ex *core.BlockFetcher,
	bcast header.Broadcaster,
	dag format.DAGService,
) *coreexchange.Listener {
	cl := coreexchange.NewListener(bcast, ex, dag)
	lc.Append(fx.Hook{
		OnStart: cl.Start,
		OnStop:  cl.Stop,
	})
	return cl
}

// RemoteClient provides a constructor for core.Client over RPC.
func RemoteClient(cfg Config) (core.Client, error) {
	return core.NewRemote(cfg.Protocol, cfg.RemoteAddr)
}
