package core

import (
	"context"
	"fmt"

	format "github.com/ipfs/go-ipld-format"
	"go.uber.org/fx"

	"github.com/celestiaorg/celestia-node/core"
	"github.com/celestiaorg/celestia-node/node/fxutil"
	"github.com/celestiaorg/celestia-node/service/header"
)

// Config combines all configuration fields for managing the relationship with a Core node.
type Config struct {
	Protocol   string
	RemoteAddr string
}

// DefaultConfig returns default configuration for Core subsystem.
func DefaultConfig() Config {
	return Config{}
}

// Components collects all the components and services related to managing the relationship with the Core node.
func Components(cfg Config) fxutil.Option {
	return fxutil.Options(
		fxutil.Provide(core.NewBlockFetcher),
		fxutil.ProvideAs(header.NewCoreExchange, new(header.Exchange)),
		fxutil.Invoke(HeaderCoreListener),
		fxutil.Provide(func(lc fx.Lifecycle) (core.Client, error) {
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

func HeaderCoreListener(
	lc fx.Lifecycle,
	ex *core.BlockFetcher,
	bcast header.Broadcaster,
	dag format.DAGService,
) *header.CoreListener {
	cl := header.NewCoreListener(bcast, ex, dag)
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
