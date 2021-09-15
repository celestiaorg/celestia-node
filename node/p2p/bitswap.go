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

// Exchange provides a constructor for IPFS block's Exchange over BitSwap.
func Exchange(cfg *Config) func (bitSwapParams) (exchange.Interface, error) {
	return func(params bitSwapParams) (exchange.Interface, error) {
		prefix := protocol.ID(fmt.Sprintf("/celestia/%s", cfg.Network))
		return bitswap.New(
			fxutil.WithLifecycle(params.Ctx, params.Lc),
			network.NewFromIpfsHost(params.Host, params.Cr, network.Prefix(prefix)),
			blockstore.NewBlockstore(params.Ds),
			bitswap.ProvideEnabled(false),

		), nil
	}
}

type bitSwapParams struct {
	fx.In

	Ctx context.Context
	Lc fx.Lifecycle
	Host host.Host
	Cr routing.ContentRouting
	Ds datastore.Batching
}
