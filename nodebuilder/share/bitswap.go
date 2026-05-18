package share

import (
	"context"
	"fmt"

	"github.com/ipfs/boxo/blockstore"
	"github.com/ipfs/boxo/exchange"
	"github.com/ipfs/go-datastore"
	ipfsmetrics "github.com/ipfs/go-metrics-interface"
	ipfsprom "github.com/ipfs/go-metrics-prometheus"
	hst "github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/fx"

	"github.com/celestiaorg/celestia-node/nodebuilder/node"
	"github.com/celestiaorg/celestia-node/nodebuilder/p2p"
	"github.com/celestiaorg/celestia-node/share/shwap/p2p/bitswap"
	"github.com/celestiaorg/celestia-node/store"
)

// dataExchange constructs Exchange(Bitswap Composition) for Shwap
func dataExchange(tp node.Type, params bitSwapParams) exchange.SessionExchange {
	prefix := protocolID(params.Net)
	net := bitswap.NewNetwork(params.Host, prefix)

	if params.PromReg != nil {
		// metrics scope is required for prometheus metrics and will be used as metrics name prefix
		params.Ctx = ipfsmetrics.CtxScope(params.Ctx, "bitswap")
		err := ipfsprom.Inject()
		if err != nil {
			return nil
		}
	}

	switch tp {
	case node.Bridge:
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

func blockstoreFromDatastore(ds datastore.Batching) (*bitswap.BlockstoreWithMetrics, error) {
	bs := blockstore.NewBlockstore(ds)
	return bitswap.NewBlockstoreWithMetrics(bs)
}

func blockstoreFromEDSStore(store *store.Store, blockStoreCacheSize int) (*bitswap.BlockstoreWithMetrics, error) {
	if blockStoreCacheSize == 0 {
		// no cache, return plain blockstore
		bs := &bitswap.Blockstore{Getter: store}
		return bitswap.NewBlockstoreWithMetrics(bs)
	}
	withCache, err := store.WithCache("blockstore", blockStoreCacheSize)
	if err != nil {
		return nil, fmt.Errorf("create cached store for blockstore:%w", err)
	}
	bs := &bitswap.Blockstore{Getter: withCache}
	return bitswap.NewBlockstoreWithMetrics(bs)
}

type bitSwapParams struct {
	fx.In

	Lifecycle fx.Lifecycle
	Ctx       context.Context
	Net       p2p.Network
	Host      hst.Host
	Bs        blockstore.Blockstore
	PromReg   prometheus.Registerer `optional:"true"`
}

func protocolID(network p2p.Network) protocol.ID {
	return protocol.ID(fmt.Sprintf("/celestia/%s", network))
}
