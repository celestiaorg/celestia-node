package p2p

import (
	"context"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/host"
	p2pconfig "github.com/libp2p/go-libp2p/config"
	"go.uber.org/fx"

	"github.com/celestiaorg/celestia-node/node/util"
)

// Host returns constructor for p2p.Host.
func Host() func(lc fx.Lifecycle, addrf p2pconfig.AddrsFactory) (host.Host, error) {
	return func(lc fx.Lifecycle, addrf p2pconfig.AddrsFactory) (host.Host, error) {
		ctx := util.LifecycleCtx(lc)
		host, err := libp2p.New(ctx,
			// do not listen automatically
			libp2p.NoListenAddrs,
			libp2p.AddrsFactory(addrf),
		)
		if err != nil {
			return nil, err
		}

		lc.Append(fx.Hook{OnStop: func(context.Context) error {
			return host.Close()
		}})

		return host, nil
	}
}
