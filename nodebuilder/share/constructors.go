package share

import (
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/routing"
	routingdisc "github.com/libp2p/go-libp2p/p2p/discovery/routing"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/getters"
	disc "github.com/celestiaorg/celestia-node/share/p2p/discovery"
	"github.com/celestiaorg/celestia-node/share/p2p/peers"
	"github.com/celestiaorg/celestia-node/share/shwap"
)

const (
	// fullNodesTag is the tag used to identify full nodes in the discovery service.
	fullNodesTag = "full/v0.0.1"
)

func newDiscovery(cfg *disc.Parameters,
) func(routing.ContentRouting, host.Host, *peers.Manager) (*disc.Discovery, error) {
	return func(
		r routing.ContentRouting,
		h host.Host,
		manager *peers.Manager,
	) (*disc.Discovery, error) {
		return disc.NewDiscovery(
			cfg,
			h,
			routingdisc.NewRoutingDiscovery(r),
			fullNodesTag,
			disc.WithOnPeersUpdate(manager.UpdateFullNodePool),
		)
	}
}

func newModule(getter share.Getter, avail share.Availability) Module {
	return &module{getter, avail}
}

func lightGetter(
	shrexGetter *getters.ShrexGetter,
	shwapGetter *shwap.Getter,
	cfg Config,
) share.Getter {
	var cascade []share.Getter
	if cfg.UseShareExchange {
		cascade = append(cascade, shrexGetter)
	}
	cascade = append(cascade, shwapGetter)
	return getters.NewCascadeGetter(cascade)
}

// ShrexGetter is added to bridge nodes for the case that a shard is removed
// after detected shard corruption. This ensures the block is fetched and stored
// by shrex the next time the data is retrieved (meaning shard recovery is
// manual after corruption is detected).
func bridgeGetter(
	storeGetter *getters.StoreGetter,
	shrexGetter *getters.ShrexGetter,
	cfg Config,
) share.Getter {
	var cascade []share.Getter
	cascade = append(cascade, storeGetter)
	if cfg.UseShareExchange {
		cascade = append(cascade, shrexGetter)
	}
	return getters.NewCascadeGetter(cascade)
}

func fullGetter(
	storeGetter *getters.StoreGetter,
	shrexGetter *getters.ShrexGetter,
	shwapGetter *shwap.Getter,
	cfg Config,
) share.Getter {
	var cascade []share.Getter
	cascade = append(cascade, storeGetter)
	if cfg.UseShareExchange {
		cascade = append(cascade, shrexGetter)
	}
	cascade = append(cascade, shwapGetter)
	return getters.NewCascadeGetter(cascade)
}
