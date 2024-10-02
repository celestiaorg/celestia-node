package p2p

import (
	"context"
	"fmt"

	"github.com/ipfs/boxo/bitswap"
	"github.com/ipfs/boxo/bitswap/network"
	"github.com/ipfs/boxo/blockstore"
	"github.com/ipfs/boxo/exchange"
	"github.com/ipfs/go-datastore"
	ipfsmetrics "github.com/ipfs/go-metrics-interface"
	ipfsprom "github.com/ipfs/go-metrics-prometheus"
	routinghelpers "github.com/libp2p/go-libp2p-routing-helpers"
	hst "github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/prometheus/client_golang/prometheus"
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
	prefix := protocolID(params.Net + "_load_test_old")
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

	ctx := params.Ctx
	if params.Metrics != nil {
		// metrics scope is required for prometheus metrics and will be used as metrics name
		// prefix
		ctx = ipfsmetrics.CtxScope(ctx, "bitswap")
	}
	bs := bitswap.New(ctx, net, params.Bs, opts...)

	params.Lifecycle.Append(fx.Hook{
		OnStop: func(_ context.Context) (err error) {
			return bs.Close()
		},
	})
	return bs
}

func blockstoreFromDatastore(
	ctx context.Context,
	ds datastore.Batching,
	b blockstoreParams,
) (blockstore.Blockstore, error) {
	if b.Metrics != nil {
		// metrics scope is required for prometheus metrics and will be used as metrics name
		// prefix
		ctx = ipfsmetrics.CtxScope(ctx, "blockstore")
	}
	bs, err := blockstore.CachedBlockstore(
		ctx,
		blockstore.NewBlockstore(ds),
		blockstore.CacheOpts{
			HasBloomFilterSize:   defaultBloomFilterSize,
			HasBloomFilterHashes: defaultBloomFilterHashes,
			HasTwoQueueCacheSize: defaultARCCacheSize,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create blockstore: %w", err)
	}
	return eds.NewBlockstoreWithMetrics(bs)
}

func blockstoreFromEDSStore(
	ctx context.Context,
	store *eds.Store,
	b blockstoreParams,
) (blockstore.Blockstore, error) {
	if b.Metrics != nil {
		// metrics scope is required for prometheus metrics and will be used as metrics name
		// prefix
		ctx = ipfsmetrics.CtxScope(ctx, "blockstore")
	}
	bs, err := blockstore.CachedBlockstore(
		ctx,
		store.Blockstore(),
		blockstore.CacheOpts{
			HasTwoQueueCacheSize: defaultARCCacheSize,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create blockstore: %w", err)
	}
	return eds.NewBlockstoreWithMetrics(bs)
}

type blockstoreParams struct {
	fx.In
	// Metrics is unused, it is in dependency graph to ensure that prometheus metrics are enabled before bitswap
	// is started.
	Metrics *bitswapMetrics `optional:"true"`
}

type bitSwapParams struct {
	fx.In

	Lifecycle fx.Lifecycle
	Ctx       context.Context
	Net       Network
	Host      hst.Host
	Bs        blockstore.Blockstore
	// Metrics is unused, it is in dependency graph to ensure that prometheus metrics are enabled before bitswap
	// is started.
	Metrics *bitswapMetrics `optional:"true"`
}

func protocolID(network Network) protocol.ID {
	return protocol.ID(fmt.Sprintf("/celestia/%s", network))
}

type bitswapMetrics struct{}

func enableBitswapMetrics(_ prometheus.Registerer) *bitswapMetrics {
	err := ipfsprom.Inject()
	if err != nil {
		log.Errorf("failed to inject bitswap metrics: %s", err)
		return nil
	}
	return &bitswapMetrics{}
}
