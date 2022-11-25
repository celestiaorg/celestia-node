package header

import (
	"testing"

	"github.com/ipfs/go-datastore"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/header/store"
	"github.com/celestiaorg/celestia-node/nodebuilder/node"
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
