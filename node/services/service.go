package services

import (
	"context"

	"github.com/ipfs/go-blockservice"
	"github.com/ipfs/go-datastore"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/peerstore"
	"github.com/libp2p/go-libp2p-core/routing"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	routingdisc "github.com/libp2p/go-libp2p/p2p/discovery/routing"
	"go.uber.org/fx"
	"go.uber.org/multierr"

	"github.com/celestiaorg/celestia-node/das"
	"github.com/celestiaorg/celestia-node/fraud"
	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/header/p2p"
	"github.com/celestiaorg/celestia-node/header/store"
	"github.com/celestiaorg/celestia-node/header/sync"
	"github.com/celestiaorg/celestia-node/libs/fxutil"
	"github.com/celestiaorg/celestia-node/params"
	headerservice "github.com/celestiaorg/celestia-node/service/header"
	"github.com/celestiaorg/celestia-node/service/share"
)

// HeaderSyncer creates a new Syncer.
func HeaderSyncer(
	ctx context.Context,
	lc fx.Lifecycle,
	ex header.Exchange,
	store header.Store,
	sub header.Subscriber,
	fsub fraud.Service,
) (*sync.Syncer, error) {
	syncer := sync.NewSyncer(ex, store, sub)
	lc.Append(fx.Hook{
		OnStart: func(context.Context) error {
			return fsub.ListenFraudProofs(fxutil.WithLifecycle(ctx, lc), fraud.BadEncoding, syncer.Start, syncer.Stop)
		},
		OnStop: syncer.Stop,
	})

	return syncer, nil
}

// P2PSubscriber creates a new p2p.Subscriber.
func P2PSubscriber(lc fx.Lifecycle, sub *pubsub.PubSub) (*p2p.Subscriber, *p2p.Subscriber) {
	p2pSub := p2p.NewSubscriber(sub)
	lc.Append(fx.Hook{
		OnStart: p2pSub.Start,
		OnStop:  p2pSub.Stop,
	})
	return p2pSub, p2pSub
}

// HeaderService creates a new header.Service.
func HeaderService(
	syncer *sync.Syncer,
	sub header.Subscriber,
	p2pServer *p2p.ExchangeServer,
	ex header.Exchange,
	store header.Store,
) *headerservice.Service {
	return headerservice.NewHeaderService(syncer, sub, p2pServer, ex, store)
}

// HeaderExchangeP2P constructs new Exchange for headers.
func HeaderExchangeP2P(cfg Config) func(params.Bootstrappers, host.Host) (header.Exchange, error) {
	return func(bpeers params.Bootstrappers, host host.Host) (header.Exchange, error) {
		peers, err := cfg.trustedPeers(bpeers)
		if err != nil {
			return nil, err
		}
		ids := make([]peer.ID, len(peers))
		for index, peer := range peers {
			ids[index] = peer.ID
			host.Peerstore().AddAddrs(peer.ID, peer.Addrs, peerstore.PermanentAddrTTL)
		}
		return p2p.NewExchange(host, ids), nil
	}
}

// HeaderP2PExchangeServer creates a new header/p2p.ExchangeServer.
func HeaderP2PExchangeServer(lc fx.Lifecycle, host host.Host, store header.Store) *p2p.ExchangeServer {
	p2pServ := p2p.NewExchangeServer(host, store)
	lc.Append(fx.Hook{
		OnStart: p2pServ.Start,
		OnStop:  p2pServ.Stop,
	})

	return p2pServ
}

// HeaderStore creates and initializes new header.Store.
func HeaderStore(lc fx.Lifecycle, ds datastore.Batching) (header.Store, error) {
	store, err := store.NewStore(ds)
	if err != nil {
		return nil, err
	}
	lc.Append(fx.Hook{
		OnStart: store.Start,
		OnStop:  store.Stop,
	})
	return store, nil
}

// HeaderStoreInit initializes the store.
func HeaderStoreInit(cfg *Config) func(context.Context, params.Network, header.Store, header.Exchange) error {
	return func(ctx context.Context, net params.Network, s header.Store, ex header.Exchange) error {
		trustedHash, err := cfg.trustedHash(net)
		if err != nil {
			return err
		}

		err = store.Init(ctx, s, ex, trustedHash)
		if err != nil {
			// TODO(@Wondertan): Error is ignored, as otherwise unit tests for Node construction fail.
			// 	This is due to requesting step of initialization, which fetches initial Header by trusted hash from
			//  the network. The step can't be done during unit tests and fixing it would require either
			//   * Having some test/dev/offline mode for Node that mocks out all the networking
			//   * Hardcoding full extended header in params pkg, instead of hashes, so we avoid requesting step
			log.Errorf("initializing store failed: %s", err)
		}

		return nil
	}
}

// ShareService constructs new share.Service.
func ShareService(
	lc fx.Lifecycle,
	bServ blockservice.BlockService,
	avail share.Availability,
	r routing.ContentRouting,
	h host.Host,
) *share.Service {
	service := share.NewService(bServ, avail)
	lc.Append(fx.Hook{
		OnStart: service.Start,
		OnStop:  service.Stop,
	})
	return service
}

// DASer constructs a new Data Availability Sampler.
func DASer(
	ctx context.Context,
	lc fx.Lifecycle,
	avail share.Availability,
	sub header.Subscriber,
	hstore header.Store,
	ds datastore.Batching,
	fservice fraud.Service,
) *das.DASer {
	das := das.NewDASer(avail, sub, hstore, ds, fservice)
	lc.Append(fx.Hook{
		OnStart: func(context.Context) error {
			return fservice.ListenFraudProofs(fxutil.WithLifecycle(ctx, lc), fraud.BadEncoding, das.Start, das.Stop)
		},
		OnStop: das.Stop,
	})

	return das
}

// FraudService constructs fraud proof service.
func FraudService(
	sub *pubsub.PubSub,
	hstore header.Store,
	ds datastore.Batching,
) (fraud.Service, fraud.Service, error) {
	f := fraud.NewService(sub, hstore.GetByHeight, ds)
	if err := f.RegisterUnmarshaler(fraud.BadEncoding, fraud.UnmarshalBEFP); err != nil {
		return nil, nil, err
	}
	return f, f, nil
}

// LightAvailability constructs light share availability wrapped in cache availability.
func LightAvailability(
	lc fx.Lifecycle,
	bServ blockservice.BlockService,
	ds datastore.Batching,
	r routing.ContentRouting,
	h host.Host,
) share.Availability {
	la := share.NewLightAvailability(bServ, routingdisc.NewRoutingDiscovery(r), h)
	ca := share.NewCacheAvailability(la, ds)
	lc.Append(fx.Hook{
		OnStart: la.Start,
		OnStop: func(ctx context.Context) error {
			laErr := la.Stop(ctx)
			caErr := ca.Close(ctx)
			return multierr.Append(laErr, caErr)
		},
	})
	return ca
}

// FullAvailability constructs full share availability wrapped in cache availability.
func FullAvailability(
	lc fx.Lifecycle,
	bServ blockservice.BlockService,
	ds datastore.Batching,
	r routing.ContentRouting,
	h host.Host,
) share.Availability {
	fa := share.NewFullAvailability(bServ, routingdisc.NewRoutingDiscovery(r), h)
	ca := share.NewCacheAvailability(fa, ds)
	lc.Append(fx.Hook{
		OnStart: fa.Start,
		OnStop: func(ctx context.Context) error {
			faErr := fa.Stop(ctx)
			caErr := ca.Close(ctx)
			return multierr.Append(faErr, caErr)
		},
	})
	return ca
}
