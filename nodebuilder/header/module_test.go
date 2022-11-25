package header

import (
	"testing"
	"time"

	"github.com/ipfs/go-datastore"
	"github.com/libp2p/go-libp2p"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/header/p2p"
	"github.com/celestiaorg/celestia-node/header/store"
	"github.com/celestiaorg/celestia-node/nodebuilder/node"
	modp2p "github.com/celestiaorg/celestia-node/nodebuilder/p2p"
)

// TestConstructModule_StoreParams ensures that all passed via functional options
// params are set in store correctly.
func TestConstructModule_StoreParams(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Store.StoreCacheSize = 15
	cfg.Store.IndexCacheSize = 25
	cfg.Store.WriteBatchSize = 35
	var headerStore *store.Store

	app := fxtest.New(t,
		fx.Provide(func() datastore.Batching {
			return datastore.NewMapDatastore()
		}),
		ConstructModule(node.Light, &cfg),
		fx.Invoke(
			func(s header.Store) {
				ss := s.(*store.Store)
				headerStore = ss
			}),
	)
	require.NoError(t, app.Err())
	require.Equal(t, headerStore.Params.StoreCacheSize, cfg.Store.StoreCacheSize)
	require.Equal(t, headerStore.Params.IndexCacheSize, cfg.Store.IndexCacheSize)
	require.Equal(t, headerStore.Params.WriteBatchSize, cfg.Store.WriteBatchSize)
}

// TestConstructModule_ExchangeParams ensures that all passed via functional options
// params are set in store correctly.
func TestConstructModule_ExchangeParams(t *testing.T) {
	cfg := DefaultConfig()
	cfg.P2PExchange.WriteDeadline = time.Minute
	cfg.P2PExchange.ReadDeadline = time.Minute * 2
	cfg.P2PExchange.MinResponses = 10
	cfg.P2PExchange.MaxRequestSize = 200
	var exchange *p2p.Exchange
	var exchangeServer *p2p.ExchangeServer

	app := fxtest.New(t,
		fx.Supply(modp2p.Private),
		fx.Supply(modp2p.Bootstrappers{}),
		fx.Provide(libp2p.New),
		fx.Provide(func() datastore.Batching {
			return datastore.NewMapDatastore()
		}),
		ConstructModule(node.Light, &cfg),
		fx.Invoke(
			func(e header.Exchange, server *p2p.ExchangeServer) {
				ex := e.(*p2p.Exchange)
				exchange = ex
				exchangeServer = server
			}),
	)
	require.NoError(t, app.Err())
	require.Equal(t, exchange.Params.WriteDeadline, cfg.P2PExchange.WriteDeadline)
	require.Equal(t, exchange.Params.ReadDeadline, cfg.P2PExchange.ReadDeadline)
	require.Equal(t, exchange.Params.MinResponses, cfg.P2PExchange.MinResponses)
	require.Equal(t, exchange.Params.MaxRequestSize, cfg.P2PExchange.MaxRequestSize)

	require.Equal(t, exchangeServer.Params.WriteDeadline, cfg.P2PExchange.WriteDeadline)
	require.Equal(t, exchangeServer.Params.ReadDeadline, cfg.P2PExchange.ReadDeadline)
	require.Equal(t, exchangeServer.Params.MinResponses, cfg.P2PExchange.MinResponses)
	require.Equal(t, exchangeServer.Params.MaxRequestSize, cfg.P2PExchange.MaxRequestSize)
}
