package p2p

import (
	"context"
	"fmt"
	"github.com/celestiaorg/celestia-node/dagblockstore"

	"github.com/ipfs/go-bitswap"
	"github.com/ipfs/go-bitswap/network"
	"github.com/ipfs/go-datastore"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	exchange "github.com/ipfs/go-ipfs-exchange-interface"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/protocol"
	routinghelpers "github.com/libp2p/go-libp2p-routing-helpers"
	"go.uber.org/fx"

	"github.com/celestiaorg/celestia-node/libs/fxutil"
	nparams "github.com/celestiaorg/celestia-node/params"
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
		bs := dagblockstore.NewDAGBlockStore(params.Ds)
		prefix := protocol.ID(fmt.Sprintf("/celestia/%s", params.Net))
		return bitswap.New(
			ctx,
			network.NewFromIpfsHost(params.Host, &routinghelpers.Null{}, network.Prefix(prefix)),
			bs,
			bitswap.ProvideEnabled(false),
			// NOTE: These below ar required for our protocol to work reliably.
			// See https://github.com/celestiaorg/celestia-node/issues/732
			bitswap.SetSendDontHaves(false),
			bitswap.SetSimulateDontHavesOnTimeout(false),
		), bs, nil
	}
}

type bitSwapParams struct {
	fx.In

	Ctx  context.Context
	Net  nparams.Network
	Lc   fx.Lifecycle
	Host host.Host
	Ds   datastore.Batching
}
