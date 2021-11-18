package services

import (
	"context"

	ipld "github.com/ipfs/go-ipld-format"
	"github.com/ipfs/go-merkledag"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"go.uber.org/fx"

	tmbytes "github.com/tendermint/tendermint/libs/bytes"

	"github.com/celestiaorg/celestia-node/das"
	"github.com/celestiaorg/celestia-node/node/fxutil"
	"github.com/celestiaorg/celestia-node/service/block"
	"github.com/celestiaorg/celestia-node/service/header"
	"github.com/celestiaorg/celestia-node/service/share"
)

func Header(lc fx.Lifecycle, ps *pubsub.PubSub, head tmbytes.HexBytes) *header.Service {
	service := header.NewHeaderService(nil, nil, ps, head)
	lc.Append(fx.Hook{
		OnStart: service.Start,
		OnStop:  service.Stop,
	})
	return service
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
func Share(lc fx.Lifecycle, dag ipld.DAGService, avail share.Availability) share.Service {
	service := share.NewService(dag, avail)
	lc.Append(fx.Hook{
		OnStart: service.Start,
		OnStop:  service.Stop,
	})
	return service
}

// DASer constructs a new Data Availability Sampler.
func DASer(avail share.Availability, service *header.Service) *das.DASer {
	return das.NewDASer(avail, service)
}

// LightAvailability constructs light share availability.
func LightAvailability(ctx context.Context, lc fx.Lifecycle, dag ipld.DAGService) share.Availability {
	return share.NewLightAvailability(merkledag.NewSession(fxutil.WithLifecycle(ctx, lc), dag))
}
