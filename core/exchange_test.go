package core

import (
	"context"
	"testing"
	"time"

	ds "github.com/ipfs/go-datastore"
	ds_sync "github.com/ipfs/go-datastore/sync"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-app/test/util/testnode"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/share/eds"
)

func TestCoreExchange_RequestHeaders(t *testing.T) {
	fetcher, _ := createCoreFetcher(t, DefaultTestConfig())

	// generate 10 blocks
	generateBlocks(t, fetcher)

	store := createStore(t)

	ce := NewExchange(fetcher, store, header.MakeExtendedHeader)
	headers, err := ce.GetRangeByHeight(context.Background(), 1, 10)
	require.NoError(t, err)

	assert.Equal(t, 10, len(headers))
}

func createCoreFetcher(t *testing.T, cfg *testnode.Config) (*BlockFetcher, testnode.Context) {
	cctx := StartTestNodeWithConfig(t, cfg)
	// wait for height 2 in order to be able to start submitting txs (this prevents
	// flakiness with accessing account state)
	_, err := cctx.WaitForHeightWithTimeout(2, time.Second*2) // TODO @renaynay: configure?
	require.NoError(t, err)
	return NewBlockFetcher(cctx.Client), cctx
}

func createStore(t *testing.T) *eds.Store {
	t.Helper()

	storeCfg := eds.DefaultParameters()
	store, err := eds.NewStore(storeCfg, t.TempDir(), ds_sync.MutexWrap(ds.NewMapDatastore()))
	require.NoError(t, err)
	return store
}

func generateBlocks(t *testing.T, fetcher *BlockFetcher) {
	sub, err := fetcher.SubscribeNewBlockEvent(context.Background())
	require.NoError(t, err)

	for i := 0; i < 10; i++ {
		<-sub
	}
}
