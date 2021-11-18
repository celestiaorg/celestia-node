package services

import (
	"context"

	"github.com/ipfs/go-datastore"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"

	"github.com/celestiaorg/celestia-node/node/p2p"

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

// Header constructs a new header.Service.
func Header(
	lc fx.Lifecycle,
	ex header.Exchange,
	store header.Store,
	ps *pubsub.PubSub,
	head tmbytes.HexBytes,
) (*header.Service, header.Broadcaster) {
	service := header.NewHeaderService(ex, store, ps, head)
	lc.Append(fx.Hook{
		OnStart: service.Start,
		OnStop:  service.Stop,
	})
	return service, service
}

func HeaderExchange(lc fx.Lifecycle, host host.Host, boot p2p.Bootstrap, store header.Store) header.Exchange {
	ex := header.NewExchange(host, peer.ID(boot), store)
	lc.Append(fx.Hook{
		OnStart: ex.Start,
		OnStop:  ex.Stop,
	})
	return ex
}

func HeaderStore(lc fx.Lifecycle, ds datastore.Batching) (header.Store, error) {
	store, err := header.NewStore(ds)
	if err != nil {
		return nil, err
	}
	lc.Append(fx.Hook{
		OnStart: store.Open,
	})
	return store, nil
}

// Block constructs new block.Service.
func Block(
	lc fx.Lifecycle,
	fetcher block.Fetcher,
	store ipld.DAGService,
	broadcaster header.Broadcaster,
) *block.Service {
	service := block.NewBlockService(fetcher, store, broadcaster)
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
