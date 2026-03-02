package share

import (
	"github.com/ipfs/boxo/blockstore"
	"github.com/ipfs/boxo/exchange"
	"go.uber.org/fx"

	headerServ "github.com/celestiaorg/celestia-node/nodebuilder/header"
	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/shwap"
	"github.com/celestiaorg/celestia-node/share/shwap/getters"
	"github.com/celestiaorg/celestia-node/share/shwap/p2p/bitswap"
	"github.com/celestiaorg/celestia-node/share/shwap/p2p/shrex/shrex_getter"
	"github.com/celestiaorg/celestia-node/store"
)

func newShareModule(getter shwap.Getter, avail share.Availability, header headerServ.Module) Module {
	return &module{getter, avail, header}
}

func bitswapGetter(
	lc fx.Lifecycle,
	exchange exchange.SessionExchange,
	bstore blockstore.Blockstore,
	wndw Window,
) *bitswap.Getter {
	getter := bitswap.NewGetter(exchange, bstore, wndw.Duration())
	lc.Append(fx.StartStopHook(getter.Start, getter.Stop))
	return getter
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
	if cfg.UseBitswap {
		cascade = append(cascade, bitswapGetter)
	}
	return getters.NewCascadeGetter(cascade)
}

// Getter is added to bridge nodes for the case where Bridge nodes are
// running in a pruned mode. This ensures the block can be retrieved from
// the network if it was pruned from the local store.
func bridgeGetter(
	storeGetter *store.Getter,
	shrexGetter *shrex_getter.Getter,
	bitswapGetter *bitswap.Getter,
	cfg Config,
) shwap.Getter {
	var cascade []shwap.Getter
	cascade = append(cascade, storeGetter)
	if cfg.UseShareExchange {
		cascade = append(cascade, shrexGetter)
	}
	if cfg.UseBitswap {
		cascade = append(cascade, bitswapGetter)
	}
	return getters.NewCascadeGetter(cascade)
}
