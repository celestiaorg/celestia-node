package header

import (
	"context"
	"testing"

	"github.com/ipfs/go-datastore"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/p2p/net/conngater"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"

	libhead "github.com/celestiaorg/go-header"
	headp2p "github.com/celestiaorg/go-header/p2p"
	"github.com/celestiaorg/go-header/store"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/libs/pidstore"
	"github.com/celestiaorg/celestia-node/nodebuilder/node"
	modp2p "github.com/celestiaorg/celestia-node/nodebuilder/p2p"
)

// TestConstructModule_StoreParams ensures that all passed via functional options
// params are set in store correctly.
func TestConstructModule_StoreParams(t *testing.T) {
	cfg := DefaultConfig(node.Light)
	cfg.Store.StoreCacheSize = 15
	cfg.Store.IndexCacheSize = 25
	cfg.Store.WriteBatchSize = 35
	var headerStore *store.Store[*header.ExtendedHeader]

	app := fxtest.New(t,
		fx.Supply(modp2p.Private),
		fx.Supply(modp2p.Bootstrappers{}),
		fx.Provide(context.Background),
		fx.Provide(libp2p.New),
		fx.Provide(conngater.NewBasicConnectionGater),
		fx.Provide(func() (datastore.Batching, datastore.Datastore) {
			ds := datastore.NewMapDatastore()
			return ds, ds
		}),
		ConstructModule[*header.ExtendedHeader](node.Light, &cfg, &modp2p.Config{}),
		fx.Invoke(
			func(s libhead.Store[*header.ExtendedHeader]) {
				ss := s.(*store.Store[*header.ExtendedHeader])
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
	cfg := DefaultConfig(node.Light)
	cfg.Client.MaxHeadersPerRangeRequest = 15
	cfg.TrustedPeers = []string{"/ip4/1.2.3.4/tcp/12345/p2p/12D3KooWNaJ1y1Yio3fFJEXCZyd1Cat3jmrPdgkYCrHfKD3Ce21p"}
	var exchange *headp2p.Exchange[*header.ExtendedHeader]
	var exchangeServer *headp2p.ExchangeServer[*header.ExtendedHeader]

	app := fxtest.New(t,
		fx.Provide(pidstore.NewPeerIDStore),
		fx.Provide(context.Background),
		fx.Supply(modp2p.Private),
		fx.Supply(modp2p.Bootstrappers{}),
		fx.Provide(libp2p.New),
		fx.Provide(func() datastore.Batching {
			return datastore.NewMapDatastore()
		}),
		ConstructModule[*header.ExtendedHeader](node.Light, &cfg, &modp2p.Config{}),
		fx.Provide(func(b datastore.Batching) (*conngater.BasicConnectionGater, error) {
			return conngater.NewBasicConnectionGater(b)
		}),
		fx.Invoke(
			func(e libhead.Exchange[*header.ExtendedHeader], server *headp2p.ExchangeServer[*header.ExtendedHeader]) {
				ex := e.(*headp2p.Exchange[*header.ExtendedHeader])
				exchange = ex
				exchangeServer = server
			}),
	)
	require.NoError(t, app.Err())
	require.Equal(t, exchange.Params.MaxHeadersPerRangeRequest, cfg.Client.MaxHeadersPerRangeRequest)
	require.Equal(t, exchange.Params.RequestTimeout, cfg.Client.RequestTimeout)

	require.Equal(t, exchangeServer.Params.WriteDeadline, cfg.Server.WriteDeadline)
	require.Equal(t, exchangeServer.Params.ReadDeadline, cfg.Server.ReadDeadline)
	require.Equal(t, exchangeServer.Params.RequestTimeout, cfg.Server.RequestTimeout)
}
