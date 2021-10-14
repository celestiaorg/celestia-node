package p2p

import (
	"context"
	"fmt"

	"github.com/ipfs/go-bitswap"
	"github.com/ipfs/go-bitswap/network"
	"github.com/ipfs/go-datastore"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	exchange "github.com/ipfs/go-ipfs-exchange-interface"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/protocol"
	"github.com/libp2p/go-libp2p-core/routing"
	"go.uber.org/fx"

	"github.com/celestiaorg/celestia-node/node/fxutil"
)

// DataExchange provides a constructor for IPFS block's DataExchange over BitSwap.
func DataExchange(cfg Config) func(bitSwapParams) (exchange.Interface, blockstore.Blockstore, error) {
	return func(params bitSwapParams) (exchange.Interface, blockstore.Blockstore, error) {
		bs, err := blockstore.CachedBlockstore(
			fxutil.WithLifecycle(params.Ctx, params.Lc),
			blockstore.NewBlockstore(params.Ds),
			blockstore.DefaultCacheOpts(),
		)
		if err != nil {
			return nil, nil, err
		}
		prefix := protocol.ID(fmt.Sprintf("/celestia/%s", cfg.Network))
		return bitswap.New(
			fxutil.WithLifecycle(params.Ctx, params.Lc),
			network.NewFromIpfsHost(params.Host, params.Cr, network.Prefix(prefix)),
			bs,
			bitswap.ProvideEnabled(false),
		), bs, nil
	}
}

type bitSwapParams struct {
	fx.In

	Ctx  context.Context
	Lc   fx.Lifecycle
	Host host.Host
	Cr   routing.ContentRouting
	Ds   datastore.Batching
}
