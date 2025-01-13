package core

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-app/v3/test/util/testnode"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/store"
)

func TestCoreExchange_RequestHeaders(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	cfg := DefaultTestConfig()
	fetcher, cctx := createCoreFetcher(t, cfg)

	generateNonEmptyBlocks(t, ctx, fetcher, cfg, cctx)

	store, err := store.NewStore(store.DefaultParameters(), t.TempDir())
	require.NoError(t, err)

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
		has, err := store.HasByHash(ctx, h.DAH.Hash())
		require.NoError(t, err)
		assert.True(t, has)

		has, err = store.HasByHeight(ctx, h.Height())
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
	fetcher, cctx := createCoreFetcher(t, cfg)

	generateNonEmptyBlocks(t, ctx, fetcher, cfg, cctx)

	store, err := store.NewStore(store.DefaultParameters(), t.TempDir())
	require.NoError(t, err)

	ce, err := NewExchange(
		fetcher,
		store,
		header.MakeExtendedHeader,
		WithAvailabilityWindow(time.Nanosecond), // all blocks will be "historic"
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
		has, err := store.HasByHeight(ctx, h.Height())
		require.NoError(t, err)
		assert.False(t, has)

		// empty EDSs are expected to exist in the store, so we skip them
		if h.DAH.Equals(share.EmptyEDSRoots()) {
			continue
		}
		has, err = store.HasByHash(ctx, h.DAH.Hash())
		require.NoError(t, err)
		assert.False(t, has)
	}
}

// TestExchange_StoreHistoricIfArchival makes sure blocks are stored past
// sampling window if archival is enabled
func TestExchange_StoreHistoricIfArchival(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	cfg := DefaultTestConfig()
	fetcher, cctx := createCoreFetcher(t, cfg)

	generateNonEmptyBlocks(t, ctx, fetcher, cfg, cctx)

	store, err := store.NewStore(store.DefaultParameters(), t.TempDir())
	require.NoError(t, err)

	ce, err := NewExchange(
		fetcher,
		store,
		header.MakeExtendedHeader,
		WithAvailabilityWindow(time.Nanosecond), // all blocks will be "historic"
		WithArchivalMode(),                      // make sure to store them anyway
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

	// ensure all "historic" EDSs were stored
	for _, h := range headers {
		has, err := store.HasByHeight(ctx, h.Height())
		require.NoError(t, err)
		assert.True(t, has)

		// empty EDSs are expected to exist in the store, so we skip them
		if h.DAH.Equals(share.EmptyEDSRoots()) {
			continue
		}
		has, err = store.HasByHash(ctx, h.DAH.Hash())
		require.NoError(t, err)
		assert.True(t, has)
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

		_, err := cctx.FillBlock(16, cfg.Genesis.Accounts()[0].Name, flags.BroadcastAsync)
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

			if bytes.Equal(share.EmptyEDSDataHash(), b.Data.Hash()) {
				continue
			}
			hashes = append(hashes, share.DataHash(b.Data.Hash()))
			i++
		case <-ctx.Done():
			t.Fatal("failed to fill blocks within timeout")
		}
	}
	generateCtxCancel()

	return hashes
}
