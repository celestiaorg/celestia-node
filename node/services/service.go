package services

import (
	"github.com/celestiaorg/celestia-node/service/block"
	"github.com/celestiaorg/celestia-node/service/share"

	ipld "github.com/ipfs/go-ipld-format"
	"go.uber.org/fx"
)

// Components wraps services creation
func Components() fx.Option {
	return fx.Options(
		fx.Provide(Block),
		fx.Provide(Share),
	)
}

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
func Share(lc fx.Lifecycle, dag ipld.DAGService) share.Service {
	service := share.NewService(dag)
	lc.Append(fx.Hook{
		OnStart: service.Start,
		OnStop:  service.Stop,
	})
	return service
}
