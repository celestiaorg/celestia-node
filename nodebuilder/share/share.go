package share

import (
	"go.uber.org/fx"

	"github.com/ipfs/go-datastore"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/routing"
	routingdisc "github.com/libp2p/go-libp2p/p2p/discovery/routing"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/availability"
	"github.com/celestiaorg/celestia-node/share/availability/cache"
	"github.com/celestiaorg/celestia-node/share/availability/discovery"
)

func Discovery(cfg Config) func(routing.ContentRouting, host.Host) (*discovery.Discovery, error) {
	return func(
		r routing.ContentRouting,
		h host.Host,
	) (*discovery.Discovery, error) {
		disc, err := discovery.NewDiscovery(
			h,
			routingdisc.NewRoutingDiscovery(r),
			availability.WithPeersLimit(cfg.PeersLimit),
			availability.WithDiscoveryInterval(cfg.DiscoveryInterval),
			availability.WithAdvertiseInterval(cfg.AdvertiseInterval),
		)
		if err != nil {
			return nil, err
		}
		return disc, nil
	}
}

// CacheAvailability wraps either Full or Light availability with a cache for result sampling.
func CacheAvailability[A share.Availability](
	lc fx.Lifecycle,
	ds datastore.Batching,
	avail A,
) (share.Availability, error) {
	ca, err := cache.NewShareAvailability(avail, ds)
	if err != nil {
		return nil, err
	}

	lc.Append(fx.Hook{
		OnStop: ca.Close,
	})

	return ca, nil
}
