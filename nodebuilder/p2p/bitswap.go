package p2p

import (
	"context"
	"errors"
	"fmt"

	"github.com/ipfs/boxo/bitswap/client"
	"github.com/ipfs/boxo/bitswap/network"
	"github.com/ipfs/boxo/bitswap/server"
	"github.com/ipfs/boxo/blockstore"
	"github.com/ipfs/boxo/exchange"
	"github.com/ipfs/go-datastore"
	routinghelpers "github.com/libp2p/go-libp2p-routing-helpers"
	hst "github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/protocol"
	"go.uber.org/fx"

	"github.com/celestiaorg/celestia-node/share/store"
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
func dataExchange(params bitSwapParams) exchange.SessionExchange {
	prefix := protocolID(params.Net)
	net := network.NewFromIpfsHost(params.Host, &routinghelpers.Null{}, network.Prefix(prefix))
	srvr := server.New(
		params.Ctx,
		net,
		params.Bs,
		server.ProvideEnabled(false), // we don't provide blocks over DHT
		// NOTE: These below are required for our protocol to work reliably.
		// // See https://github.com/celestiaorg/celestia-node/issues/732
		server.SetSendDontHaves(false),
	)

	clnt := client.New(
		params.Ctx,
		net,
		params.Bs,
		client.WithBlockReceivedNotifier(srvr),
		client.SetSimulateDontHavesOnTimeout(false),
		client.WithoutDuplicatedBlockStats(),
	)
	net.Start(srvr, clnt) // starting with hook does not work

	params.Lifecycle.Append(fx.Hook{
		OnStop: func(ctx context.Context) (err error) {
			err = errors.Join(err, clnt.Close())
			err = errors.Join(err, srvr.Close())
			net.Stop()
			return err
		},
	})

	return clnt
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

func blockstoreFromEDSStore(ctx context.Context, s *store.Store, ds datastore.Batching) (blockstore.Blockstore, error) {
	return blockstore.CachedBlockstore(
		ctx,
		store.NewBlockstore(s, ds),
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
