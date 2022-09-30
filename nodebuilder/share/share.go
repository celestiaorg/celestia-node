package share

import (
	"github.com/ipfs/go-datastore"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/routing"
	routingdisc "github.com/libp2p/go-libp2p/p2p/discovery/routing"
	"go.uber.org/fx"

	"github.com/celestiaorg/celestia-node/share"
)

func Discovery(cfg Config) func(routing.ContentRouting, host.Host) *share.Discovery {
	return func(
		r routing.ContentRouting,
		h host.Host,
	) *share.Discovery {
		return share.NewDiscovery(
			h,
			routingdisc.NewRoutingDiscovery(r),
			cfg.PeersLimit,
			cfg.DiscoveryInterval,
			cfg.AdvertiseInterval,
		)
	}
}

// CacheAvailability wraps either Full or Light availability with a cache for result sampling.
func CacheAvailability[A share.Availability](lc fx.Lifecycle, ds datastore.Batching, avail A) share.Availability {
	ca := share.NewCacheAvailability(avail, ds)
	lc.Append(fx.Hook{
		OnStop: ca.Close,
	})
	return ca
}
