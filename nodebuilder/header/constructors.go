package header

import (
	"context"

	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/peerstore"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/p2p/net/conngater"
	"go.uber.org/fx"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/header/p2p"
	"github.com/celestiaorg/celestia-node/header/store"
	"github.com/celestiaorg/celestia-node/header/sync"
	modp2p "github.com/celestiaorg/celestia-node/nodebuilder/p2p"
)

// newP2PServer constructs a new ExchangeServer using the given Network as a protocolID suffix.
func newP2PServer(
	host host.Host,
	store header.Store,
	opts []p2p.Option[p2p.ServerParameters],
) (*p2p.ExchangeServer, error) {
	return p2p.NewExchangeServer(host, store, opts...)
}

// newP2PExchange constructs a new Exchange for headers.
func newP2PExchange(cfg Config) func(
	fx.Lifecycle,
	modp2p.Bootstrappers,
	host.Host,
	*conngater.BasicConnectionGater,
	[]p2p.Option[p2p.ClientParameters],
) (header.Exchange, error) {
	return func(
		lc fx.Lifecycle,
		bpeers modp2p.Bootstrappers,
		host host.Host,
		conngater *conngater.BasicConnectionGater,
		opts []p2p.Option[p2p.ClientParameters],
	) (header.Exchange, error) {
		peers, err := cfg.trustedPeers(bpeers)
		if err != nil {
			return nil, err
		}
		ids := make([]peer.ID, len(peers))
		for index, peer := range peers {
			ids[index] = peer.ID
			host.Peerstore().AddAddrs(peer.ID, peer.Addrs, peerstore.PermanentAddrTTL)
		}
		exchange, err := p2p.NewExchange(host, ids, conngater, opts...)
		if err != nil {
			return nil, err
		}
		lc.Append(fx.Hook{
			OnStart: func(ctx context.Context) error {
				return exchange.Start(ctx)
			},
			OnStop: func(ctx context.Context) error {
				return exchange.Stop(ctx)
			},
		})
		return exchange, nil
	}
}

// newSyncer constructs new Syncer for headers.
func newSyncer(ex header.Exchange, store InitStore, sub header.Subscriber, opts []sync.Options) (*sync.Syncer, error) {
	return sync.NewSyncer(ex, store, sub, opts...)
}

// InitStore is a type representing initialized header store.
// NOTE: It is needed to ensure that Store is always initialized before Syncer is started.
type InitStore header.Store

// newInitStore constructs an initialized store
func newInitStore(
	lc fx.Lifecycle,
	cfg Config,
	net modp2p.Network,
	s header.Store,
	ex header.Exchange,
) (InitStore, error) {
	trustedHash, err := cfg.trustedHash(net)
	if err != nil {
		return nil, err
	}

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			return store.Init(ctx, s, ex, trustedHash)
		},
	})

	return s, nil
}

func newSubscriber(ps *pubsub.PubSub, network modp2p.Network) *p2p.Subscriber {
	return p2p.NewSubscriber(ps, string(network))
}
