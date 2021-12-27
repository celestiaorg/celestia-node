package services

import (
	"context"
	"github.com/ipfs/go-datastore"
	ipld "github.com/ipfs/go-ipld-format"
	"github.com/ipfs/go-merkledag"
	"github.com/libp2p/go-libp2p-core/host"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"go.uber.org/fx"

	"github.com/celestiaorg/celestia-node/das"
	"github.com/celestiaorg/celestia-node/node/fxutil"
	"github.com/celestiaorg/celestia-node/service/block"
	"github.com/celestiaorg/celestia-node/service/header"
	"github.com/celestiaorg/celestia-node/service/share"
)

// HeaderSyncer creates a new header.Syncer.
func HeaderSyncer(cfg Config) func(ex header.Exchange, store header.Store) (*header.Syncer, error) {
	return func(ex header.Exchange, store header.Store) (*header.Syncer, error) {
		trustedHash, err := cfg.trustedHash()
		if err != nil {
			return nil, err
		}

		return header.NewSyncer(ex, store, trustedHash), nil
	}
}

// PubsubManager creates a new header.PubsubManager.
func PubsubManager(sub *pubsub.PubSub, syncer *header.Syncer) *header.PubsubManager {
	return header.NewPubsubManager(sub, syncer.Validate)
}

// HeaderService creates a new header.Service.
func HeaderService(lc fx.Lifecycle,
	syncer *header.Syncer,
	psManager *header.PubsubManager,
	ex header.Exchange,
	) *header.Service {

	headerLifecycles := []header.Lifecycle{
		syncer,
		psManager,
		ex,
	}
	// if core is enabled, add core listener
	if _, ok := ex.(*header.CoreExchange); ok {
		// TODO @renaynay: implement this
	}

	headerServ := header.NewHeaderService(headerLifecycles)
	lc.Append(fx.Hook{
		OnStart: headerServ.Start,
		OnStop:  headerServ.Stop,
	})

	return headerServ
}

// HeaderExchangeP2P constructs new P2PExchange for headers.
func HeaderExchangeP2P(cfg Config) func(
	lc fx.Lifecycle,
	host host.Host,
	store header.Store,
) (header.Exchange, error) {
	return func(lc fx.Lifecycle, host host.Host, store header.Store) (header.Exchange, error) {
		peer, err := cfg.trustedPeer()
		if err != nil {
			return nil, err
		}

		ex := header.NewP2PExchange(host, peer, store)
		lc.Append(fx.Hook{
			OnStart: ex.Start,
			OnStop:  ex.Stop,
		})
		return ex, nil
	}
}

func StartHeaderExchangeP2PServer(host host.Host, store header.Store) *header.P2PServer {
	return header.NewP2PServer(host, store)
}

// HeaderStore creates new header.Store.
func HeaderStore(ds datastore.Batching) (header.Store, error) {
	return header.NewStore(ds)
}

// BlockService constructs new block.Service.
func BlockService(
	lc fx.Lifecycle,
	store ipld.DAGService,
) *block.Service {
	service := block.NewBlockService(store)
	lc.Append(fx.Hook{
		OnStart: service.Start,
		OnStop:  service.Stop,
	})
	return service
}

// ShareService constructs new share.Service.
func ShareService(lc fx.Lifecycle, dag ipld.DAGService, avail share.Availability) share.Service {
	service := share.NewService(dag, avail)
	lc.Append(fx.Hook{
		OnStart: service.Start,
		OnStop:  service.Stop,
	})
	return service
}

// DASer constructs a new Data Availability Sampler.
func DASer(lc fx.Lifecycle, avail share.Availability, sub header.Subscriber) *das.DASer {
	das := das.NewDASer(avail, sub)
	lc.Append(fx.Hook{
		OnStart: das.Start,
		OnStop:  das.Stop,
	})
	return das
}

// LightAvailability constructs light share availability.
func LightAvailability(ctx context.Context, lc fx.Lifecycle, dag ipld.DAGService) share.Availability {
	return share.NewLightAvailability(merkledag.NewSession(fxutil.WithLifecycle(ctx, lc), dag))
}
