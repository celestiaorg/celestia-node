package share

import (
	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/shwap"
	"github.com/celestiaorg/celestia-node/share/shwap/getters"
	"github.com/celestiaorg/celestia-node/share/shwap/p2p/bitswap"
	"github.com/celestiaorg/celestia-node/share/shwap/p2p/shrex/shrex_getter"
	"github.com/celestiaorg/celestia-node/store"
)

func newShareModule(getter shwap.Getter, avail share.Availability) Module {
	return &module{getter, avail}
}

func lightGetter(
	shrexGetter *shrex_getter.Getter,
	bitswapGetter *bitswap.Getter,
	cfg Config,
) shwap.Getter {
	var cascade []shwap.Getter
	if cfg.UseShareExchange {
		cascade = append(cascade, shrexGetter)
	}
	cascade = append(cascade, bitswapGetter)
	return getters.NewCascadeGetter(cascade)
}

// Getter is added to bridge nodes for the case that a shard is removed
// after detected shard corruption. This ensures the block is fetched and stored
// by shrex the next time the data is retrieved (meaning shard recovery is
// manual after corruption is detected).
func bridgeGetter(
	storeGetter *store.Getter,
	shrexGetter *shrex_getter.Getter,
	cfg Config,
) shwap.Getter {
	var cascade []shwap.Getter
	cascade = append(cascade, storeGetter)
	if cfg.UseShareExchange {
		cascade = append(cascade, shrexGetter)
	}
	return getters.NewCascadeGetter(cascade)
}

func fullGetter(
	storeGetter *store.Getter,
	shrexGetter *shrex_getter.Getter,
	cfg Config,
) shwap.Getter {
	var cascade []shwap.Getter
	cascade = append(cascade, storeGetter)
	if cfg.UseShareExchange {
		cascade = append(cascade, shrexGetter)
	}
	return getters.NewCascadeGetter(cascade)
}
