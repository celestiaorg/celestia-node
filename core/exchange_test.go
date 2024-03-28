package core

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/cosmos/cosmos-sdk/client/flags"
	ds "github.com/ipfs/go-datastore"
	ds_sync "github.com/ipfs/go-datastore/sync"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-app/test/util/testnode"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/pruner"
	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds"
)

func TestCoreExchange_RequestHeaders(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	cfg := DefaultTestConfig()
	cfg.ChainID = networkID
	fetcher, cctx := createCoreFetcher(t, cfg)

	// generate several blocks
	generateBlocks(t, fetcher, cfg, cctx)

	store := createStore(t)

	ce, err := NewExchange(fetcher, store, header.MakeExtendedHeader)
	require.NoError(t, err)

	// initialize store with genesis block
	genHeight := int64(1)
	genBlock, err := fetcher.GetBlock(ctx, &genHeight)
	require.NoError(t, err)
	genHeader, err := ce.Get(ctx, genBlock.Header.Hash().Bytes())
	require.NoError(t, err)

	to := uint64(40) // ensures some blocks will be non-empty
	expectedFirstHeightInRange := genHeader.Height() + 1
	expectedLastHeightInRange := to - 1
	expectedLenHeaders := to - expectedFirstHeightInRange

	// request headers from height 1 to 10 [2:35)
	headers, err := ce.GetRangeByHeight(context.Background(), genHeader, to)
	require.NoError(t, err)

	assert.Len(t, headers, int(expectedLenHeaders))
	assert.Equal(t, expectedFirstHeightInRange, headers[0].Height())
	assert.Equal(t, expectedLastHeightInRange, headers[len(headers)-1].Height())

	for _, h := range headers {
		has, err := store.Has(ctx, h.DAH.Hash())
		require.NoError(t, err)
		assert.True(t, has)
	}
}

// TestExchange_DoNotStoreHistoric tests that the CoreExchange will not
// store EDSs that are historic if pruning is enabled.
func TestExchange_DoNotStoreHistoric(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	cfg := DefaultTestConfig()
	cfg.ChainID = networkID
	fetcher, cctx := createCoreFetcher(t, cfg)

	// generate 10 blocks
	generateBlocks(t, fetcher, cfg, cctx)

	store := createStore(t)

	ce, err := NewExchange(
		fetcher,
		store,
		header.MakeExtendedHeader,
		WithAvailabilityWindow(pruner.AvailabilityWindow(time.Nanosecond)), // all blocks will be "historic"
	)
	require.NoError(t, err)

	// initialize store with genesis block
	genHeight := int64(1)
	genBlock, err := fetcher.GetBlock(ctx, &genHeight)
	require.NoError(t, err)
	genHeader, err := ce.Get(ctx, genBlock.Header.Hash().Bytes())
	require.NoError(t, err)

	// ensures some blocks will be non-empty
	headers, err := ce.GetRangeByHeight(ctx, genHeader, 40)
	require.NoError(t, err)

	// ensure none of the "historic" EDSs were stored
	for _, h := range headers {
		if bytes.Equal(h.DataHash, share.EmptyRoot().Hash()) {
			continue
		}
		has, err := store.Has(ctx, h.DAH.Hash())
		require.NoError(t, err)
		assert.False(t, has)
	}
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

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err = store.Start(ctx)
	require.NoError(t, err)

	// store an empty square to initialize EDS store
	eds := share.EmptyExtendedDataSquare()
	err = store.Put(ctx, share.EmptyRoot().Hash(), eds)
	require.NoError(t, err)

	t.Cleanup(func() {
		err = store.Stop(ctx)
		require.NoError(t, err)
	})

	return store
}

func generateBlocks(t *testing.T, fetcher *BlockFetcher, cfg *testnode.Config, cctx testnode.Context) {
	sub, err := fetcher.SubscribeNewBlockEvent(context.Background())
	require.NoError(t, err)

	i := 0
	for i < 20 {
		_, err := cctx.FillBlock(16, cfg.Accounts, flags.BroadcastBlock)
		require.NoError(t, err)

		b := <-sub
		if bytes.Equal(b.Header.DataHash, share.EmptyRoot().Hash()) {
			continue
		}
		i++
	}
}
