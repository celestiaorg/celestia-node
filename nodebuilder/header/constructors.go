package header

import (
	"context"

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

	"github.com/celestiaorg/celestia-node/header"
	modfraud "github.com/celestiaorg/celestia-node/nodebuilder/fraud"
	modp2p "github.com/celestiaorg/celestia-node/nodebuilder/p2p"
	"github.com/celestiaorg/celestia-node/share/eds/byzantine"
)

// newP2PExchange constructs a new Exchange for headers.
func newP2PExchange(
	lc fx.Lifecycle,
	bpeers modp2p.Bootstrappers,
	network modp2p.Network,
	chainID modp2p.ChainID,
	host host.Host,
	conngater *conngater.BasicConnectionGater,
	cfg Config,
) (libhead.Exchange[*header.ExtendedHeader], error) {
	peers, err := cfg.trustedPeers(bpeers)
	if err != nil {
		return nil, err
	}
	ids := make([]peer.ID, len(peers))
	for index, peer := range peers {
		ids[index] = peer.ID
		host.Peerstore().AddAddrs(peer.ID, peer.Addrs, peerstore.PermanentAddrTTL)
	}
	exchange, err := p2p.NewExchange[*header.ExtendedHeader](host, ids, conngater,
		p2p.WithParams(cfg.Client),
		p2p.WithNetworkID[p2p.ClientParameters](network.String()),
		p2p.WithChainID(chainID.String()),
	)
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
func newSyncer(
	ex libhead.Exchange[*header.ExtendedHeader],
	fservice libfraud.Service,
	store InitStore,
	sub libhead.Subscriber[*header.ExtendedHeader],
	cfg Config,
) (*sync.Syncer[*header.ExtendedHeader], *modfraud.ServiceBreaker[*sync.Syncer[*header.ExtendedHeader]], error) {
	syncer, err := sync.NewSyncer[*header.ExtendedHeader](ex, store, sub,
		sync.WithParams(cfg.Syncer),
		sync.WithBlockTime(modp2p.BlockTime),
	)
	if err != nil {
		return nil, nil, err
	}

	return syncer, &modfraud.ServiceBreaker[*sync.Syncer[*header.ExtendedHeader]]{
		Service:   syncer,
		FraudType: byzantine.BadEncoding,
		FraudServ: fservice,
	}, nil
}

// InitStore is a type representing initialized header store.
// NOTE: It is needed to ensure that Store is always initialized before Syncer is started.
type InitStore libhead.Store[*header.ExtendedHeader]

// newInitStore constructs an initialized store
func newInitStore(
	lc fx.Lifecycle,
	cfg Config,
	net modp2p.Network,
	s libhead.Store[*header.ExtendedHeader],
	ex libhead.Exchange[*header.ExtendedHeader],
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
