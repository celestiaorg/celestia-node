package services

import (
	"github.com/celestiaorg/celestia-node/service/block"

	ipld "github.com/ipfs/go-ipld-format"
	"go.uber.org/fx"
)

// Components wraps services creation
func Components() fx.Option {
	return fx.Options(
		fx.Provide(func(lc fx.Lifecycle, fetcher block.Fetcher, store ipld.DAGService) *block.Service {
			service := block.NewBlockService(fetcher, store)
			lc.Append(fx.Hook{
				OnStart: service.Start,
				OnStop:  service.Stop,
			})
			return service
		}),
	)
}
