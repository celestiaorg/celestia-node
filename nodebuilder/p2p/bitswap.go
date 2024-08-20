package p2p

import (
	"context"
	"fmt"

	"github.com/ipfs/boxo/bitswap"
	"github.com/ipfs/boxo/bitswap/network"
	"github.com/ipfs/boxo/blockstore"
	"github.com/ipfs/boxo/exchange"
	"github.com/ipfs/go-datastore"
	routinghelpers "github.com/libp2p/go-libp2p-routing-helpers"
	hst "github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/protocol"
	"go.uber.org/fx"

	"github.com/celestiaorg/celestia-node/share/eds"
)

const (
	// default size of bloom filter in blockStore
	defaultBloomFilterSize = 512 << 10
	// default amount of hash functions defined for bloom filter
	defaultBloomFilterHashes = 7
	// default size of arc cache in blockStore
	defaultARCCacheSize = 64 << 10
)

// dataExchange provides a constructor for IPFS block's DataExchange over BitSwap.
func dataExchange(params bitSwapParams) exchange.Interface {
	prefix := protocolID(params.Net)
	net := network.NewFromIpfsHost(params.Host, &routinghelpers.Null{}, network.Prefix(prefix))

	opts := []bitswap.Option{
		// Server options
		bitswap.ProvideEnabled(false), // we don't provide blocks over DHT
		// NOTE: These below are required for our protocol to work reliably.
		// // See https://github.com/celestiaorg/celestia-node/issues/732
		bitswap.SetSendDontHaves(false),

		// Client options
		bitswap.SetSimulateDontHavesOnTimeout(false),
		bitswap.WithoutDuplicatedBlockStats(),
	}
	bs := bitswap.New(params.Ctx, net, params.Bs, opts...)

	params.Lifecycle.Append(fx.Hook{
		OnStop: func(_ context.Context) (err error) {
			return bs.Close()
		},
	})
	return bs
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

func blockstoreFromEDSStore(ctx context.Context, store *eds.Store) (blockstore.Blockstore, error) {
	return blockstore.CachedBlockstore(
		ctx,
		store.Blockstore(),
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
