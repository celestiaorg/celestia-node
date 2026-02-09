package header

import (
	"context"

	"github.com/ipfs/go-datastore"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/peerstore"
	"github.com/libp2p/go-libp2p/p2p/net/conngater"
	"go.uber.org/fx"

	libhead "github.com/celestiaorg/go-header"
	"github.com/celestiaorg/go-header/p2p"
	"github.com/celestiaorg/go-header/store"
	"github.com/celestiaorg/go-header/sync"

	"github.com/celestiaorg/celestia-node/nodebuilder/node"
	modp2p "github.com/celestiaorg/celestia-node/nodebuilder/p2p"
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
) (*p2p.Exchange[H], error) {
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
	ndtp node.Type,
	net modp2p.Network,
	ex libhead.Exchange[H],
	store libhead.Store[H],
	sub libhead.Subscriber[H],
	cfg Config,
	isArchival node.ArchivalMode,
) (*sync.Syncer[H], error) {
	switch ndtp {
	case node.Bridge:
		// Bridge nodes: check if archival mode is enabled via --archival flag
		if isArchival {
			// Archival mode: disable header pruning and sync from genesis
			genesis, err := modp2p.GenesisFor(net)
			if err != nil {
				return nil, err
			}
			cfg.Syncer.PruningWindow = 0
			cfg.Syncer.SyncFromHash = genesis
			if genesis == "" {
				cfg.Syncer.SyncFromHeight = 1
			}
		}
	case node.Light:
	default:
		panic("invalid node type")
	}

	opts := []sync.Option{
		sync.WithParams(cfg.Syncer),
		sync.WithBlockTime(modp2p.BlockTime),
		sync.WithTrustingPeriod(trustingPeriod),
	}
	if MetricsEnabled {
		opts = append(opts, sync.WithMetrics())
	}

	syncer, err := sync.NewSyncer[H](ex, store, sub, opts...)
	if err != nil {
		return nil, err
	}

	return syncer, nil
}

// newStore constructs an initialized store
func newStore[H libhead.Header[H]](
	lc fx.Lifecycle,
	cfg Config,
	ds datastore.Batching,
) (libhead.Store[H], error) {
	opts := []store.Option{store.WithParams(cfg.Store)}
	if MetricsEnabled {
		opts = append(opts, store.WithMetrics())
	}

	s, err := store.NewStore[H](ds, opts...)
	if err != nil {
		return nil, err
	}

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			return s.Start(ctx)
		},
		OnStop: func(ctx context.Context) error {
			return s.Stop(ctx)
		},
	})

	return s, nil
}
