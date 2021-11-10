package services

import (
	"context"

	"github.com/ipfs/go-merkledag"

	"github.com/celestiaorg/celestia-node/node/fxutil"
	"github.com/celestiaorg/celestia-node/service/block"
	"github.com/celestiaorg/celestia-node/service/share"

	ipld "github.com/ipfs/go-ipld-format"
	"go.uber.org/fx"
)

// Block constructs new block.Service.
func Block(lc fx.Lifecycle, fetcher block.Fetcher, store ipld.DAGService) *block.Service {
	service := block.NewBlockService(fetcher, store)
	lc.Append(fx.Hook{
		OnStart: service.Start,
		OnStop:  service.Stop,
	})
	return service
}

// Share constructs new share.Service.
func Share(ctx context.Context, lc fx.Lifecycle, dag ipld.DAGService) share.Service {
	avail := share.NewLightAvailability(merkledag.NewSession(fxutil.WithLifecycle(ctx, lc), dag))
	service := share.NewService(dag, avail)
	lc.Append(fx.Hook{
		OnStart: service.Start,
		OnStop:  service.Stop,
	})
	return service
}
