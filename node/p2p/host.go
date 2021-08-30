package p2p

import (
	"context"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/host"
	"go.uber.org/fx"

	"github.com/celestiaorg/celestia-node/node/util"
)

func Host(lc fx.Lifecycle) (host.Host, error) {
	ctx := util.LifecycleCtx(lc)
	host, err := libp2p.New(ctx)
	if err != nil {
		return nil, err
	}

	lc.Append(fx.Hook{OnStop: func(context.Context) error {
		return host.Close()
	}})

	return host, nil
}
