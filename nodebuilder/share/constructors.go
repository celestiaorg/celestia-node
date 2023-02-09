package share

import (
	"context"
	"errors"

	"github.com/filecoin-project/dagstore"
	"github.com/ipfs/go-datastore"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/routing"
	routingdisc "github.com/libp2p/go-libp2p/p2p/discovery/routing"
	"github.com/libp2p/go-libp2p/p2p/net/conngater"
	"go.uber.org/fx"

	"github.com/celestiaorg/celestia-node/header"
	libhead "github.com/celestiaorg/celestia-node/libs/header"
	modp2p "github.com/celestiaorg/celestia-node/nodebuilder/p2p"
	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/availability/cache"
	disc "github.com/celestiaorg/celestia-node/share/availability/discovery"
	"github.com/celestiaorg/celestia-node/share/eds"
	"github.com/celestiaorg/celestia-node/share/getters"
	"github.com/celestiaorg/celestia-node/share/p2p/peers"
	"github.com/celestiaorg/celestia-node/share/p2p/shrexsub"

	"github.com/celestiaorg/celestia-app/pkg/da"
)

func discovery(cfg Config) func(routing.ContentRouting, host.Host) *disc.Discovery {
	return func(
		r routing.ContentRouting,
		h host.Host,
	) *disc.Discovery {
		return disc.NewDiscovery(
			h,
			routingdisc.NewRoutingDiscovery(r),
			cfg.PeersLimit,
			cfg.DiscoveryInterval,
			cfg.AdvertiseInterval,
		)
	}
}

// cacheAvailability wraps either Full or Light availability with a cache for result sampling.
func cacheAvailability[A share.Availability](lc fx.Lifecycle, ds datastore.Batching, avail A) share.Availability {
	ca := cache.NewShareAvailability(avail, ds)
	lc.Append(fx.Hook{
		OnStop: ca.Close,
	})
	return ca
}

func newModule(getter share.Getter, avail share.Availability) Module {
	return &module{getter, avail}
}

// ensureEmptyCARExists adds an empty EDS to the provided EDS store.
func ensureEmptyCARExists(ctx context.Context, store *eds.Store) error {
	emptyEDS := share.EmptyExtendedDataSquare()
	emptyDAH := da.NewDataAvailabilityHeader(emptyEDS)

	err := store.Put(ctx, emptyDAH.Hash(), emptyEDS)
	if errors.Is(err, dagstore.ErrShardExists) {
		return nil
	}
	return err
}

func peerManager(
	headerSub libhead.Subscriber[*header.ExtendedHeader],
	shrexSub *shrexsub.PubSub,
	discovery *disc.Discovery,
	host host.Host,
	connGater *conngater.BasicConnectionGater,
) *peers.Manager {
	// TODO: find better syncTimeout duration?
	return peers.NewManager(headerSub, shrexSub, discovery, host, connGater, modp2p.BlockTime)
}

func fullGetter(
	store *eds.Store,
	shrexGetter *getters.ShrexGetter,
	ipldGetter *getters.IPLDGetter,
) share.Getter {
	return getters.NewCascadeGetter(
		[]share.Getter{
			getters.NewStoreGetter(store),
			getters.NewTeeGetter(shrexGetter, store),
			getters.NewTeeGetter(ipldGetter, store),
		},
		// TODO: What constant should we use here?
		modp2p.BlockTime,
	)
}
