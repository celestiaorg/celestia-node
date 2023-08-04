package share

import (
	"context"
	"errors"

	"github.com/filecoin-project/dagstore"
	"github.com/ipfs/go-blockservice"
	"github.com/ipfs/go-datastore"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/routing"
	routingdisc "github.com/libp2p/go-libp2p/p2p/discovery/routing"
	"go.uber.org/fx"

	"github.com/celestiaorg/celestia-app/pkg/da"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/availability/cache"
	"github.com/celestiaorg/celestia-node/share/availability/light"
	"github.com/celestiaorg/celestia-node/share/eds"
	"github.com/celestiaorg/celestia-node/share/getters"
	"github.com/celestiaorg/celestia-node/share/ipld"
	disc "github.com/celestiaorg/celestia-node/share/p2p/discovery"
)

func newDiscovery(cfg Config) func(routing.ContentRouting, host.Host) *disc.Discovery {
	return func(
		r routing.ContentRouting,
		h host.Host,
	) *disc.Discovery {
		return disc.NewDiscovery(
			h,
			routingdisc.NewRoutingDiscovery(r),
			disc.WithPeersLimit(cfg.Discovery.PeersLimit),
			disc.WithAdvertiseInterval(cfg.Discovery.AdvertiseInterval),
		)
	}
}

// cacheAvailability wraps light availability with a cache for result sampling.
func cacheAvailability(lc fx.Lifecycle, ds datastore.Batching, avail *light.ShareAvailability) share.Availability {
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
	emptyDAH, err := da.NewDataAvailabilityHeader(emptyEDS)
	if err != nil {
		return err
	}

	err = store.Put(ctx, emptyDAH.Hash(), emptyEDS)
	if errors.Is(err, dagstore.ErrShardExists) {
		return nil
	}
	return err
}

// ensureEmptyEDSInBS checks if the given DAG contains an empty block data square.
// If it does not, it stores an empty block. This optimization exists to prevent
// redundant storing of empty block data so that it is only stored once and returned
// upon request for a block with an empty data square.
func ensureEmptyEDSInBS(ctx context.Context, bServ blockservice.BlockService) error {
	_, err := ipld.AddShares(ctx, share.EmptyBlockShares(), bServ)
	return err
}

func lightGetter(
	shrexGetter *getters.ShrexGetter,
	ipldGetter *getters.IPLDGetter,
	cfg Config,
) share.Getter {
	var cascade []share.Getter
	if cfg.UseShareExchange {
		cascade = append(cascade, shrexGetter)
	}
	cascade = append(cascade, ipldGetter)
	return getters.NewCascadeGetter(cascade)
}

func fullGetter(
	store *eds.Store,
	storeGetter *getters.StoreGetter,
	shrexGetter *getters.ShrexGetter,
	ipldGetter *getters.IPLDGetter,
	cfg Config,
) share.Getter {
	var cascade []share.Getter
	cascade = append(cascade, storeGetter)
	if cfg.UseShareExchange {
		cascade = append(cascade, getters.NewTeeGetter(shrexGetter, store))
	}
	cascade = append(cascade, getters.NewTeeGetter(ipldGetter, store))
	return getters.NewCascadeGetter(cascade)
}
