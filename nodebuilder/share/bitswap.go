package share

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
	"github.com/celestiaorg/celestia-node/nodebuilder/p2p"
	"github.com/celestiaorg/celestia-node/share/shwap/p2p/bitswap"
	"github.com/celestiaorg/celestia-node/store"
)

const (
	// TODO(@walldiss): expose cache size to cfg
	// default blockstore cache size
	defaultBlockstoreCacheSize = 128
)

// dataExchange constructs Exchange(Bitswap Composition) for Shwap
func dataExchange(tp node.Type, params bitSwapParams) exchange.SessionExchange {
	prefix := protocolID(params.Net)
	net := bitswap.NewNetwork(params.Host, prefix)

	switch tp {
	case node.Full, node.Bridge:
		bs := bitswap.New(params.Ctx, net, params.Bs)
		net.Start(bs.Client, bs.Server)
		params.Lifecycle.Append(fx.Hook{
			OnStop: func(_ context.Context) (err error) {
				net.Stop()
				return bs.Close()
			},
		})
		return bs
	case node.Light:
		cl := bitswap.NewClient(params.Ctx, net, params.Bs)
		net.Start(cl)
		params.Lifecycle.Append(fx.Hook{
			OnStop: func(_ context.Context) (err error) {
				net.Stop()
				return cl.Close()
			},
		})
		return cl
	default:
		panic(fmt.Sprintf("unsupported node type: %v", tp))
	}
}

func blockstoreFromDatastore(ds datastore.Batching) (blockstore.Blockstore, error) {
	return blockstore.NewBlockstore(ds), nil
}

func blockstoreFromEDSStore(store *store.Store) (blockstore.Blockstore, error) {
	withCache, err := store.WithCache("blockstore", defaultBlockstoreCacheSize)
	if err != nil {
		return nil, fmt.Errorf("create cached store for blockstore:%w", err)
	}
	bs := &bitswap.Blockstore{Getter: withCache}
	return bs, nil
}

type bitSwapParams struct {
	fx.In

	Lifecycle fx.Lifecycle
	Ctx       context.Context
	Net       p2p.Network
	Host      hst.Host
	Bs        blockstore.Blockstore
}

func protocolID(network p2p.Network) protocol.ID {
	return protocol.ID(fmt.Sprintf("/celestia/%s", network))
}
