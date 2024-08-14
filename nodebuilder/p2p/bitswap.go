package p2p

import (
	"context"
	"fmt"

	"github.com/ipfs/boxo/blockstore"
	"github.com/ipfs/boxo/exchange"
	"github.com/ipfs/go-datastore"
	hst "github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/protocol"
	"go.uber.org/fx"

	"github.com/celestiaorg/celestia-node/nodebuilder/node"
	"github.com/celestiaorg/celestia-node/share/shwap/p2p/bitswap"
	"github.com/celestiaorg/celestia-node/store"
)

const (
	// default size of bloom filter in blockStore
	defaultBloomFilterSize = 512 << 10
	// default amount of hash functions defined for bloom filter
	defaultBloomFilterHashes = 7
	// default size of arc cache in blockStore
	defaultARCCacheSize = 64 << 10
	// TODO(@walldiss): expose cache size to cfg
	// default blockstore cache size
	defaultBlockstoreCacheSize = 128
)

// dataExchange provides a constructor for IPFS block's DataExchange over BitSwap.
func dataExchange(tp node.Type, params bitSwapParams) exchange.SessionExchange {
	prefix := protocolID(params.Net)
	net := bitswap.NewNetwork(params.Host, prefix)

	switch tp {
	case node.Full, node.Bridge:
		bs := bitswap.New(params.Ctx, net, params.Bs)
		params.Lifecycle.Append(fx.Hook{
			OnStop: func(_ context.Context) (err error) {
				bs.Close()
				return err
			},
		})
		return bs
	case node.Light:
		cl := bitswap.NewClient(params.Ctx, net, params.Bs)
		net.Start(cl)
		params.Lifecycle.Append(fx.Hook{
			OnStop: func(_ context.Context) (err error) {
				net.Stop()
				cl.Close()
				return err
			},
		})
		return cl
	default:
		panic(fmt.Sprintf("unsupported node type: %v", tp))
	}
}

func blockstoreFromDatastore(ctx context.Context, ds datastore.Batching) (blockstore.Blockstore, error) {
	return blockstore.CachedBlockstore(
		ctx,
		blockstore.NewBlockstore(ds),
		blockstore.CacheOpts{
			HasBloomFilterSize:   defaultBloomFilterSize,
			HasBloomFilterHashes: defaultBloomFilterHashes,
			HasTwoQueueCacheSize: defaultARCCacheSize,
		},
	)
}

func blockstoreFromEDSStore(ctx context.Context, store *store.Store) (blockstore.Blockstore, error) {
	withCache, err := store.WithCache("blockstore", defaultBlockstoreCacheSize)
	if err != nil {
		return nil, fmt.Errorf("create cached store for blockstore:%w", err)
	}
	bs := &bitswap.Blockstore{Getter: withCache}
	return blockstore.CachedBlockstore(
		ctx,
		bs,
		blockstore.CacheOpts{
			HasTwoQueueCacheSize: defaultARCCacheSize,
		},
	)
}

type bitSwapParams struct {
	fx.In

	Lifecycle fx.Lifecycle
	Ctx       context.Context
	Net       Network
	Host      hst.Host
	Bs        blockstore.Blockstore
}

func protocolID(network Network) protocol.ID {
	return protocol.ID(fmt.Sprintf("/celestia/%s", network))
}
