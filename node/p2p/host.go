package p2p

import (
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/host"
	"go.uber.org/fx"

	"github.com/celestiaorg/celestia-node/node/util"
)

func Host(lc fx.Lifecycle) (host.Host, error) {
	ctx := util.LifecycleCtx(lc)
	return libp2p.New(ctx)
}
