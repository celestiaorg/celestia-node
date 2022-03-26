package services

import (
	"context"

	"github.com/ipfs/go-datastore"
	ipld "github.com/ipfs/go-ipld-format"
	"github.com/ipfs/go-merkledag"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/peerstore"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"go.uber.org/fx"

	"github.com/celestiaorg/celestia-node/das"
	"github.com/celestiaorg/celestia-node/node/fxutil"
	"github.com/celestiaorg/celestia-node/service/header"
	"github.com/celestiaorg/celestia-node/service/share"
)

// HeaderSyncer creates a new header.Syncer.
func HeaderSyncer(
	lc fx.Lifecycle,
	ex header.Exchange,
	store header.Store,
	sub header.Subscriber,
) (*header.Syncer, error) {
	syncer := header.NewSyncer(ex, store, sub)
	lc.Append(fx.Hook{
		OnStart: syncer.Start,
		OnStop:  syncer.Stop,
	})
	return syncer, nil
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
	sub header.Subscriber,
	p2pServer *header.P2PExchangeServer,
	ex header.Exchange,
) *header.Service {
	return header.NewHeaderService(syncer, sub, p2pServer, ex)
}

// HeaderExchangeP2P constructs new P2PExchange for headers.
func HeaderExchangeP2P(cfg Config) func(host host.Host) (header.Exchange, error) {
	return func(host host.Host) (header.Exchange, error) {
		peers, err := cfg.trustedPeers()
		if err != nil {
			return nil, err
		}
		ids := make([]peer.ID, len(peers))
		for index, peer := range peers {
			ids[index] = peer.ID
			host.Peerstore().AddAddrs(peer.ID, peer.Addrs, peerstore.PermanentAddrTTL)
		}
		return header.NewP2PExchange(host, ids), nil
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

// HeaderStore creates and initializes new header.Store.
func HeaderStore(lc fx.Lifecycle, ds datastore.Batching) (header.Store, error) {
	store, err := header.NewStore(ds)
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
func HeaderStoreInit(cfg *Config) func(context.Context, header.Store, header.Exchange) error {
	return func(ctx context.Context, store header.Store, ex header.Exchange) error {
		trustedHash, err := cfg.trustedHash()
		if err != nil {
			return err
		}

		err = header.InitStore(ctx, store, ex, trustedHash)
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

func FullAvailability(dag ipld.DAGService) share.Availability {
	return share.NewFullAvailability(dag)
}
