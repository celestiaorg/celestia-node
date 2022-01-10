package services

import (
	"context"

	"github.com/ipfs/go-datastore"
	ipld "github.com/ipfs/go-ipld-format"
	"github.com/ipfs/go-merkledag"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peerstore"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"go.uber.org/fx"

	"github.com/celestiaorg/celestia-node/das"
	"github.com/celestiaorg/celestia-node/node/fxutil"
	"github.com/celestiaorg/celestia-node/service/block"
	"github.com/celestiaorg/celestia-node/service/header"
	"github.com/celestiaorg/celestia-node/service/share"
)

// HeaderSyncer creates a new header.Syncer.
func HeaderSyncer(cfg Config) func(
	lc fx.Lifecycle,
	ex header.Exchange,
	store header.Store,
	sub *header.P2PSubscriber,
) (*header.Syncer, error) {
	return func(
		lc fx.Lifecycle,
		ex header.Exchange,
		store header.Store,
		sub *header.P2PSubscriber,
	) (*header.Syncer, error) {
		trustedHash, err := cfg.trustedHash()
		if err != nil {
			return nil, err
		}

		syncer := header.NewSyncer(ex, store, sub, trustedHash)
		lc.Append(fx.Hook{
			OnStart: syncer.Start,
		})

		return syncer, nil
	}
}

// P2PSubscriber creates a new header.P2PSubscriber.
func P2PSubscriber(lc fx.Lifecycle, sub *pubsub.PubSub) *header.P2PSubscriber {
	p2pSub := header.NewP2PSubscriber(sub)
	lc.Append(fx.Hook{
		OnStart: p2pSub.Start,
		OnStop:  p2pSub.Stop,
	})
	return p2pSub
}

// HeaderService creates a new header.Service.
func HeaderService(
	syncer *header.Syncer,
	p2pSub *header.P2PSubscriber,
	p2pServer *header.P2PExchangeServer,
	ex header.Exchange,
) *header.Service {
	return header.NewHeaderService(syncer, p2pSub, p2pServer, ex)
}

// HeaderExchangeP2P constructs new P2PExchange for headers.
func HeaderExchangeP2P(cfg Config) func(host host.Host) (header.Exchange, error) {
	return func(host host.Host) (header.Exchange, error) {
		peer, err := cfg.trustedPeer()
		if err != nil {
			return nil, err
		}

		host.Peerstore().AddAddrs(peer.ID, peer.Addrs, peerstore.PermanentAddrTTL)
		return header.NewP2PExchange(host, peer.ID), nil
	}
}

// HeaderP2PExchangeServer creates a new header.P2PExchangeServer.
func HeaderP2PExchangeServer(lc fx.Lifecycle, host host.Host, store header.Store) *header.P2PExchangeServer {
	p2pServ := header.NewP2PExchangeServer(host, store)
	lc.Append(fx.Hook{
		OnStart: p2pServ.Start,
		OnStop:  p2pServ.Stop,
	})

	return p2pServ
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

func FullAvailability(ctx context.Context, lc fx.Lifecycle, dag ipld.DAGService) share.Availability {
	return share.NewFullAvailability(merkledag.NewSession(fxutil.WithLifecycle(ctx, lc), dag))
}
