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
	cfg.ChainID = testChainID
	fetcher, cctx := createCoreFetcher(t, cfg)

	generateNonEmptyBlocks(t, ctx, fetcher, cfg, cctx)

	store := createStore(t)

	ce, err := NewExchange(fetcher, store, header.MakeExtendedHeader)
	require.NoError(t, err)

	// initialize store with genesis block
	genHeight := int64(1)
	genBlock, err := fetcher.GetBlock(ctx, &genHeight)
	require.NoError(t, err)
	genHeader, err := ce.Get(ctx, genBlock.Header.Hash().Bytes())
	require.NoError(t, err)

	to := uint64(30)
	expectedFirstHeightInRange := genHeader.Height() + 1
	expectedLastHeightInRange := to - 1
	expectedLenHeaders := to - expectedFirstHeightInRange

	// request headers from height 1 to 20 [2:30)
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
	cfg.ChainID = testChainID
	fetcher, cctx := createCoreFetcher(t, cfg)

	generateNonEmptyBlocks(t, ctx, fetcher, cfg, cctx)

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

	headers, err := ce.GetRangeByHeight(ctx, genHeader, 30)
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

// fillBlocks fills blocks until the context is canceled.
func fillBlocks(
	t *testing.T,
	ctx context.Context,
	cfg *testnode.Config,
	cctx testnode.Context,
) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		_, err := cctx.FillBlock(16, cfg.Accounts, flags.BroadcastBlock)
		require.NoError(t, err)
	}
}

// generateNonEmptyBlocks generates at least 20 non-empty blocks
func generateNonEmptyBlocks(
	t *testing.T,
	ctx context.Context,
	fetcher *BlockFetcher,
	cfg *testnode.Config,
	cctx testnode.Context,
) []share.DataHash {
	// generate several non-empty blocks
	generateCtx, generateCtxCancel := context.WithCancel(context.Background())

	sub, err := fetcher.SubscribeNewBlockEvent(ctx)
	require.NoError(t, err)
	defer func() {
		err = fetcher.UnsubscribeNewBlockEvent(ctx)
		require.NoError(t, err)
	}()

	go fillBlocks(t, generateCtx, cfg, cctx)

	hashes := make([]share.DataHash, 0, 20)

	i := 0
	for i < 20 {
		select {
		case b, ok := <-sub:
			require.True(t, ok)

			if !bytes.Equal(b.Data.Hash(), share.EmptyRoot().Hash()) {
				hashes = append(hashes, share.DataHash(b.Data.Hash()))
				i++
			}
		case <-ctx.Done():
			t.Fatal("failed to fill blocks within timeout")
		}
	}
	generateCtxCancel()

	return hashes
}
