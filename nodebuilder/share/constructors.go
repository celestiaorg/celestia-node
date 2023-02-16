package share

import (
	"context"
	"errors"
	"time"

	"github.com/filecoin-project/dagstore"
	"github.com/ipfs/go-datastore"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/routing"
	routingdisc "github.com/libp2p/go-libp2p/p2p/discovery/routing"
	"go.uber.org/fx"

	"github.com/celestiaorg/celestia-app/pkg/da"
	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/availability/cache"
	disc "github.com/celestiaorg/celestia-node/share/availability/discovery"
	"github.com/celestiaorg/celestia-node/share/eds"
	"github.com/celestiaorg/celestia-node/share/getters"
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

func fullGetter(
	store *eds.Store,
	shrexGetter *getters.ShrexGetter,
	ipldGetter *getters.IPLDGetter,
	cfg Config,
) share.Getter {
	var cascade []share.Getter
	// based on the default value of das.SampleTimeout
	timeout := time.Minute
	cascade = append(cascade, getters.NewStoreGetter(store))
	if cfg.UseShareExchange {
		// if we are using share exchange, we split the timeout between the two getters
		// once async cascadegetter is implemented, we can remove this
		timeout /= 2
		cascade = append(cascade, getters.NewTeeGetter(shrexGetter, store))
	}
	cascade = append(cascade, getters.NewTeeGetter(ipldGetter, store))

	return getters.NewCascadeGetter(cascade, timeout)
}
