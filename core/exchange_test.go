package core

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-app/v7/test/util/testnode"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/store"
)

func TestCoreExchange_RequestHeaders(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	cfg := DefaultTestConfig()
	fetcher, cctx := createCoreFetcher(t, cfg)
	t.Cleanup(func() {
		require.NoError(t, cctx.Stop())
	})
	nonEmptyBlocks := generateNonEmptyBlocks(t, ctx, fetcher, cfg, cctx.Context)

	store, err := store.NewStore(store.DefaultParameters(), t.TempDir())
	require.NoError(t, err)

	ce, err := NewExchange(fetcher, store, header.MakeExtendedHeader)
	require.NoError(t, err)

	// initialize store with genesis block
	genHeight := int64(1)
	genBlock, err := fetcher.GetBlock(ctx, genHeight)
	require.NoError(t, err)
	genHeader, err := ce.Get(ctx, genBlock.Header.Hash().Bytes())
	require.NoError(t, err)

	to := nonEmptyBlocks[len(nonEmptyBlocks)-1].height
	expectedFirstHeightInRange := genHeader.Height() + 1
	expectedLastHeightInRange := to - 1
	expectedLenHeaders := to - expectedFirstHeightInRange

	// request headers from height 1 to 20 [2:30)
	_, err = cctx.WaitForHeightWithTimeout(int64(to), 10*time.Second)
	require.NoError(t, err)

	// request headers from height 1 to last non-empty block height [2:to)
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
	t.Cleanup(func() {
		require.NoError(t, cctx.Stop())
	})
	nonEmptyBlocks := generateNonEmptyBlocks(t, ctx, fetcher, cfg, cctx.Context)

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
	genBlock, err := fetcher.GetBlock(ctx, genHeight)
	require.NoError(t, err)
	genHeader, err := ce.Get(ctx, genBlock.Header.Hash().Bytes())
	require.NoError(t, err)

	to := nonEmptyBlocks[len(nonEmptyBlocks)-1].height

	err = cctx.WaitForBlocks(int64(to))
	require.NoError(t, err)

	headers, err := ce.GetRangeByHeight(ctx, genHeader, to)
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
	t.Cleanup(func() {
		require.NoError(t, cctx.Stop())
	})
	nonEmptyBlocks := generateNonEmptyBlocks(t, ctx, fetcher, cfg, cctx.Context)

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
	genBlock, err := fetcher.GetBlock(ctx, genHeight)
	require.NoError(t, err)
	genHeader, err := ce.Get(ctx, genBlock.Header.Hash().Bytes())
	require.NoError(t, err)

	to := nonEmptyBlocks[len(nonEmptyBlocks)-1].height

	_, err = cctx.WaitForHeight(int64(to))
	require.NoError(t, err)

	headers, err := ce.GetRangeByHeight(ctx, genHeader, to)
	require.NoError(t, err)

	// ensure all "historic" EDSs were stored but not the .q4 files
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

		// ensure .q4 file was not stored
		has, err = store.HasQ4ByHash(ctx, h.DAH.Hash())
		require.NoError(t, err)
		assert.False(t, has)
	}
}

func createCoreFetcher(t *testing.T, cfg *testnode.Config) (*BlockFetcher, *Network) {
	t.Helper()

	// Retry up to 3 times to handle transient "address already in use" failures.
	// These occur because MustGetFreePort uses UDP probing, but the actual listeners
	// bind TCP — another process can grab the port in the window between the two.
	const maxRetries = 3
	var network *Network
	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			reallocateTestNodePorts(cfg)
		}
		network = NewNetwork(t, cfg)
		if err := network.Start(); err != nil {
			if attempt < maxRetries-1 && isAddressInUseError(err) {
				t.Logf("port conflict on attempt %d, retrying with new ports", attempt+1)
				_ = network.Stop()
				continue
			}
			require.NoError(t, err)
		}
		break
	}

	t.Cleanup(func() {
		require.NoError(t, network.Stop())
	})
	// wait for height 2 in order to be able to start submitting txs (this prevents
	// flakiness with accessing account state)
	_, err := network.WaitForHeightWithTimeout(2, time.Second*2) // TODO @renaynay: configure?
	require.NoError(t, err)
	fetcher, err := NewBlockFetcher(network.GRPCClient)
	require.NoError(t, err)
	return fetcher, network
}

// isAddressInUseError reports whether the error is a "bind: address already in use" error.
func isAddressInUseError(err error) bool {
	return err != nil && strings.Contains(err.Error(), "address already in use")
}

// reallocateTestNodePorts assigns fresh free ports to all network addresses in cfg,
// replacing the ones allocated at config-creation time that may now be taken.
func reallocateTestNodePorts(cfg *testnode.Config) {
	cfg.TmConfig.RPC.ListenAddress = fmt.Sprintf("tcp://127.0.0.1:%d", testnode.MustGetFreePort())
	cfg.TmConfig.P2P.ListenAddress = fmt.Sprintf("tcp://127.0.0.1:%d", testnode.MustGetFreePort())
	cfg.TmConfig.RPC.GRPCListenAddress = fmt.Sprintf("tcp://127.0.0.1:%d", testnode.MustGetFreePort())
	cfg.AppConfig.GRPC.Address = fmt.Sprintf("127.0.0.1:%d", testnode.MustGetFreePort())
	cfg.AppConfig.API.Address = fmt.Sprintf("tcp://127.0.0.1:%d", testnode.MustGetFreePort())
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

type testBlocks struct {
	height   uint64
	datahash share.DataHash
}

// generateNonEmptyBlocks generates at least 20 non-empty blocks
func generateNonEmptyBlocks(
	t *testing.T,
	ctx context.Context,
	fetcher *BlockFetcher,
	cfg *testnode.Config,
	cctx testnode.Context,
) []testBlocks {
	// generate several non-empty blocks
	generateCtx, generateCtxCancel := context.WithCancel(context.Background())

	sub, err := fetcher.SubscribeNewBlockEvent(generateCtx)
	require.NoError(t, err)

	go fillBlocks(t, generateCtx, cfg, cctx)

	filledBlocks := make([]testBlocks, 0, 20)

	i := 0
	for i < 20 {
		select {
		case b, ok := <-sub:
			require.True(t, ok)

			if bytes.Equal(share.EmptyEDSDataHash(), b.Data.Hash()) {
				continue
			}
			filledBlocks = append(filledBlocks, testBlocks{
				height:   uint64(b.Header.Height),
				datahash: share.DataHash(b.Data.Hash()),
			})
			i++
		case <-ctx.Done():
			t.Fatal("failed to fill blocks within timeout")
		}
	}
	generateCtxCancel()

	return filledBlocks
}
