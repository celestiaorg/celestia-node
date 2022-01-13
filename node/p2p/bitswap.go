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

	"github.com/celestiaorg/celestia-node/build"

	"github.com/celestiaorg/celestia-node/node/fxutil"
)

const (
	// default size of bloom filter in blockStore
	defaultBloomFilterSize = 512 << 10
	// default amount of hash functions defined for bloom filter
	defaultBloomFilterHashes = 7
	// default size of arc cache in blockStore
	defaultARCCacheSize = 64 << 10
)

// DataExchange provides a constructor for IPFS block's DataExchange over BitSwap.
func DataExchange(cfg Config) func(bitSwapParams) (exchange.Interface, blockstore.Blockstore, error) {
	return func(params bitSwapParams) (exchange.Interface, blockstore.Blockstore, error) {
		ctx := fxutil.WithLifecycle(params.Ctx, params.Lc)
		bs, err := blockstore.CachedBlockstore(
			ctx,
			blockstore.NewBlockstore(params.Ds),
			blockstore.CacheOpts{
				HasBloomFilterSize:   defaultBloomFilterSize,
				HasBloomFilterHashes: defaultBloomFilterHashes,
				HasARCCacheSize:      defaultARCCacheSize,
			},
		)
		if err != nil {
			return nil, nil, err
		}
		prefix := protocol.ID(fmt.Sprintf("/celestia/%s", build.GetNetwork()))
		return bitswap.New(
			ctx,
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
