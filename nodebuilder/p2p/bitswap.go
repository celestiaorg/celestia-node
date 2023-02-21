package p2p

import (
	"context"
	"fmt"

	"github.com/ipfs/go-bitswap"
	"github.com/ipfs/go-bitswap/network"
	"github.com/ipfs/go-datastore"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	exchange "github.com/ipfs/go-ipfs-exchange-interface"
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
	prefix := protocol.ID(fmt.Sprintf("/celestia/%s", params.Net))
	return bitswap.New(
		params.Ctx,
		network.NewFromIpfsHost(params.Host, &routinghelpers.Null{}, network.Prefix(prefix)),
		params.Bs,
		bitswap.ProvideEnabled(false),
		// NOTE: These below ar required for our protocol to work reliably.
		// See https://github.com/celestiaorg/celestia-node/issues/732
		bitswap.SetSendDontHaves(false),
		bitswap.SetSimulateDontHavesOnTimeout(false),
	)
}

func blockstoreFromDatastore(ctx context.Context, ds datastore.Batching) (blockstore.Blockstore, error) {
	return blockstore.CachedBlockstore(
		ctx,
		blockstore.NewBlockstore(ds),
		blockstore.CacheOpts{
			HasBloomFilterSize:   defaultBloomFilterSize,
			HasBloomFilterHashes: defaultBloomFilterHashes,
			HasARCCacheSize:      defaultARCCacheSize,
		},
	)
}

func blockstoreFromEDSStore(ctx context.Context, store *eds.Store) (blockstore.Blockstore, error) {
	return blockstore.CachedBlockstore(
		ctx,
		store.Blockstore(),
		blockstore.CacheOpts{
			HasARCCacheSize: defaultARCCacheSize,
		},
	)
}

type bitSwapParams struct {
	fx.In

	Ctx  context.Context
	Net  Network
	Host hst.Host
	Bs   blockstore.Blockstore
}
