package header

import (
	"context"

	"github.com/ipfs/go-datastore"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/peerstore"
	"github.com/libp2p/go-libp2p/p2p/net/conngater"
	"go.uber.org/fx"

	libfraud "github.com/celestiaorg/go-fraud"
	libhead "github.com/celestiaorg/go-header"
	"github.com/celestiaorg/go-header/p2p"
	"github.com/celestiaorg/go-header/store"
	"github.com/celestiaorg/go-header/sync"

	modfraud "github.com/celestiaorg/celestia-node/nodebuilder/fraud"
	modp2p "github.com/celestiaorg/celestia-node/nodebuilder/p2p"
	"github.com/celestiaorg/celestia-node/share/eds/byzantine"
)

// newP2PExchange constructs a new Exchange for headers.
func newP2PExchange[H libhead.Header[H]](
	lc fx.Lifecycle,
	cfg Config,
	bpeers modp2p.Bootstrappers,
	network modp2p.Network,
	host host.Host,
	conngater *conngater.BasicConnectionGater,
	pidstore p2p.PeerIDStore,
) (libhead.Exchange[H], error) {
	peers, err := cfg.trustedPeers(bpeers)
	if err != nil {
		return nil, err
	}
	ids := make([]peer.ID, len(peers))
	for index, peer := range peers {
		ids[index] = peer.ID
		host.Peerstore().AddAddrs(peer.ID, peer.Addrs, peerstore.PermanentAddrTTL)
	}

	opts := []p2p.Option[p2p.ClientParameters]{
		p2p.WithParams(cfg.Client),
		p2p.WithNetworkID[p2p.ClientParameters](network.String()),
		p2p.WithChainID(network.String()),
		p2p.WithPeerIDStore[p2p.ClientParameters](pidstore),
	}
	if MetricsEnabled {
		opts = append(opts, p2p.WithMetrics[p2p.ClientParameters]())
	}

	exchange, err := p2p.NewExchange[H](host, ids, conngater, opts...)
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

// newSyncer constructs new Syncer for headers.
func newSyncer[H libhead.Header[H]](
	ex libhead.Exchange[H],
	fservice libfraud.Service[H],
	store libhead.Store[H],
	sub libhead.Subscriber[H],
	cfg Config,
) (*sync.Syncer[H], *modfraud.ServiceBreaker[*sync.Syncer[H], H], error) {
	opts := []sync.Option{sync.WithParams(cfg.Syncer), sync.WithBlockTime(modp2p.BlockTime)}
	if MetricsEnabled {
		opts = append(opts, sync.WithMetrics())
	}

	syncer, err := sync.NewSyncer[H](ex, store, sub, opts...)
	if err != nil {
		return nil, nil, err
	}

	return syncer, &modfraud.ServiceBreaker[*sync.Syncer[H], H]{
		Service:   syncer,
		FraudType: byzantine.BadEncoding,
		FraudServ: fservice,
	}, nil
}

// newInitStore constructs an initialized store
func newInitStore[H libhead.Header[H]](
	lc fx.Lifecycle,
	cfg Config,
	net modp2p.Network,
	ds datastore.Batching,
	ex libhead.Exchange[H],
) (libhead.Store[H], error) {
	s, err := store.NewStore[H](ds, store.WithParams(cfg.Store))
	if err != nil {
		return nil, err
	}

	if MetricsEnabled {
		err = libhead.WithMetrics[H](s)
		if err != nil {
			return nil, err
		}
	}

	trustedHash, err := cfg.trustedHash(net)
	if err != nil {
		return nil, err
	}

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			err = store.Init[H](ctx, s, ex, trustedHash)
			if err != nil {
				return err
			}
			return s.Start(ctx)
		},
		OnStop: func(ctx context.Context) error {
			return s.Stop(ctx)
		},
	})

	return s, nil
}
