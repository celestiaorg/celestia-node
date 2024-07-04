package pruner

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/sync"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/header/headertest"
)

/*
	| toPrune  | availability window |
*/

// TestService tests the pruner service to check whether the expected
// amount of blocks are pruned within a given AvailabilityWindow.
// This test runs a pruning cycle once which should prune at least
// 2 blocks (as the AvailabilityWindow is ~2 blocks). Since the
// prune-able header determination is time-based, it cannot be
// exact.
func TestService(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	blockTime := time.Millisecond

	// all headers generated in suite are timestamped to time.Now(), so
	// they will all be considered "pruneable" within the availability window (
	suite := headertest.NewTestSuite(t, 1, blockTime)
	store := headertest.NewCustomStore(t, suite, 20)

	mp := &mockPruner{}

	serv, err := NewService(
		mp,
		AvailabilityWindow(time.Millisecond*2),
		store,
		sync.MutexWrap(datastore.NewMapDatastore()),
		blockTime,
	)
	require.NoError(t, err)

	serv.ctx, serv.cancel = ctx, cancel

	err = serv.loadCheckpoint(ctx)
	require.NoError(t, err)

	time.Sleep(time.Millisecond * 2)

	lastPruned, err := serv.lastPruned(ctx)
	require.NoError(t, err)
	lastPruned = serv.prune(ctx, lastPruned)

	assert.Greater(t, lastPruned.Height(), uint64(2))
	assert.Greater(t, serv.checkpoint.LastPrunedHeight, uint64(2))
}

// TestService_FailedAreRecorded checks whether the pruner service
// can accurately detect blocks to be pruned and store them
// to checkpoint.
func TestService_FailedAreRecorded(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	blockTime := time.Millisecond

	// all headers generated in suite are timestamped to time.Now(), so
	// they will all be considered "pruneable" within the availability window
	suite := headertest.NewTestSuite(t, 1, blockTime)
	store := headertest.NewCustomStore(t, suite, 100)

	mp := &mockPruner{
		failHeight: map[uint64]int{4: 0, 5: 0, 13: 0},
	}

	serv, err := NewService(
		mp,
		AvailabilityWindow(time.Millisecond*20),
		store,
		sync.MutexWrap(datastore.NewMapDatastore()),
		blockTime,
	)
	require.NoError(t, err)

	serv.ctx = ctx

	err = serv.loadCheckpoint(ctx)
	require.NoError(t, err)

	// ensures at least 13 blocks are prune-able
	time.Sleep(time.Millisecond * 50)

	// trigger a prune job
	lastPruned, err := serv.lastPruned(ctx)
	require.NoError(t, err)
	_ = serv.prune(ctx, lastPruned)

	assert.Len(t, serv.checkpoint.FailedHeaders, 3)
	for expectedFail := range mp.failHeight {
		_, exists := serv.checkpoint.FailedHeaders[expectedFail]
		assert.True(t, exists)
	}

	// trigger another prune job, which will prioritize retrying
	// failed blocks
	lastPruned, err = serv.lastPruned(ctx)
	require.NoError(t, err)
	_ = serv.prune(ctx, lastPruned)

	assert.Len(t, serv.checkpoint.FailedHeaders, 0)
}

func TestServiceCheckpointing(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	store := headertest.NewStore(t)

	mp := &mockPruner{}

	serv, err := NewService(
		mp,
		AvailabilityWindow(time.Second),
		store,
		sync.MutexWrap(datastore.NewMapDatastore()),
		time.Millisecond,
	)
	require.NoError(t, err)

	err = serv.loadCheckpoint(ctx)
	require.NoError(t, err)

	// ensure checkpoint was initialized correctly
	assert.Equal(t, uint64(1), serv.checkpoint.LastPrunedHeight)
	assert.Empty(t, serv.checkpoint.FailedHeaders)

	// update checkpoint
	err = serv.updateCheckpoint(ctx, uint64(3), map[uint64]struct{}{2: {}})
	require.NoError(t, err)

	// ensure checkpoint was updated correctly in datastore
	err = serv.loadCheckpoint(ctx)
	require.NoError(t, err)
	assert.Equal(t, uint64(3), serv.checkpoint.LastPrunedHeight)
	assert.Len(t, serv.checkpoint.FailedHeaders, 1)
}

// TestPrune_LargeNumberOfBlocks tests that the pruner service with a large
// number of blocks to prune (an archival node turning into a pruned node) is
// able to prune the blocks in one prune cycle.
func TestPrune_LargeNumberOfBlocks(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	var maxHeadersPerLoopOld int
	maxHeadersPerLoopOld, maxHeadersPerLoop = maxHeadersPerLoop, 10
	t.Cleanup(func() { maxHeadersPerLoop = maxHeadersPerLoopOld })

	blockTime := time.Nanosecond
	availabilityWindow := AvailabilityWindow(blockTime * 10)

	// all headers generated in suite are timestamped to time.Now(), so
	// they will all be considered "pruneable" within the availability window
	suite := headertest.NewTestSuite(t, 1, blockTime)
	store := headertest.NewCustomStore(t, suite, maxHeadersPerLoop*6) // add small buffer

	mp := &mockPruner{failHeight: make(map[uint64]int, 0)}

	serv, err := NewService(
		mp,
		availabilityWindow,
		store,
		sync.MutexWrap(datastore.NewMapDatastore()),
		blockTime,
	)
	require.NoError(t, err)
	serv.ctx = ctx

	err = serv.loadCheckpoint(ctx)
	require.NoError(t, err)

	// ensures availability window has passed
	time.Sleep(availabilityWindow.Duration() + time.Millisecond*100)

	// trigger a prune job
	lastPruned, err := serv.lastPruned(ctx)
	require.NoError(t, err)
	_ = serv.prune(ctx, lastPruned)

	// ensure all headers have been pruned
	assert.Equal(t, uint64(maxHeadersPerLoop*5), serv.checkpoint.LastPrunedHeight)
	assert.Len(t, serv.checkpoint.FailedHeaders, 0)
}

func TestFindPruneableHeaders(t *testing.T) {
	testCases := []struct {
		name           string
		availWindow    AvailabilityWindow
		blockTime      time.Duration
		startTime      time.Time
		headerAmount   int
		expectedLength int
	}{
		{
			name: "Estimated range matches expected",
			// Availability window is one week
			availWindow: AvailabilityWindow(time.Hour * 24 * 7),
			blockTime:   time.Hour,
			// Make two weeks of headers
			headerAmount: 2 * (24 * 7),
			startTime:    time.Now().Add(-2 * time.Hour * 24 * 7),
			// One week of headers are pruneable
			expectedLength: (24 * 7) + 1,
		},
		{
			name: "Estimated range not sufficient but finds the correct tail",
			// Availability window is one week
			availWindow: AvailabilityWindow(time.Hour * 24 * 7),
			blockTime:   time.Hour,
			// Make three weeks of headers
			headerAmount: 3 * (24 * 7),
			startTime:    time.Now().Add(-3 * time.Hour * 24 * 7),
			// Two weeks of headers are pruneable
			expectedLength: (2 * 24 * 7) + 1,
		},
		{
			name: "No pruneable headers",
			// Availability window is two weeks
			availWindow: AvailabilityWindow(2 * time.Hour * 24 * 7),
			blockTime:   time.Hour,
			// Make one week of headers
			headerAmount: 24 * 7,
			startTime:    time.Now().Add(-time.Hour * 24 * 7),
			// No headers are pruneable
			expectedLength: 0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			t.Cleanup(cancel)

			headerGenerator := NewSpacedHeaderGenerator(t, tc.startTime, tc.blockTime)
			store := headertest.NewCustomStore(t, headerGenerator, tc.headerAmount)

			mp := &mockPruner{}

			serv, err := NewService(
				mp,
				tc.availWindow,
				store,
				sync.MutexWrap(datastore.NewMapDatastore()),
				tc.blockTime,
			)
			require.NoError(t, err)

			err = serv.Start(ctx)
			require.NoError(t, err)

			lastPruned, err := serv.lastPruned(ctx)
			require.NoError(t, err)

			pruneable, err := serv.findPruneableHeaders(ctx, lastPruned)
			require.NoError(t, err)
			require.Len(t, pruneable, tc.expectedLength)

			pruneableCutoff := time.Now().Add(-tc.availWindow.Duration())
			// All returned headers are older than the availability window
			for _, h := range pruneable {
				require.WithinRange(t, h.Time(), tc.startTime, pruneableCutoff)
			}

			// The next header after the last pruneable header is too new to prune
			if len(pruneable) != 0 {
				lastPruneable := pruneable[len(pruneable)-1]
				if lastPruneable.Height() != store.Height() {
					firstUnpruneable, err := store.GetByHeight(ctx, lastPruneable.Height()+1)
					require.NoError(t, err)
					require.WithinRange(t, firstUnpruneable.Time(), pruneableCutoff, time.Now())
				}
			}
		})
	}
}

type mockPruner struct {
	deletedHeaderHashes []pruned

	// tells the mockPruner on which heights to fail
	failHeight map[uint64]int
}

type pruned struct {
	hash   string
	height uint64
}

func (mp *mockPruner) Prune(_ context.Context, h *header.ExtendedHeader) error {
	for fail := range mp.failHeight {
		if h.Height() == fail {
			// if retried, return successful
			if mp.failHeight[fail] > 0 {
				return nil
			}
			mp.failHeight[fail]++
			return fmt.Errorf("failed to prune")
		}
	}
	mp.deletedHeaderHashes = append(mp.deletedHeaderHashes, pruned{hash: h.Hash().String(), height: h.Height()})
	return nil
}

// TODO @renaynay @distractedm1nd: Deduplicate via headertest utility.
// https://github.com/celestiaorg/celestia-node/issues/3278.
type SpacedHeaderGenerator struct {
	t                  *testing.T
	TimeBetweenHeaders time.Duration
	currentTime        time.Time
	currentHeight      int64
}

func NewSpacedHeaderGenerator(
	t *testing.T, startTime time.Time, timeBetweenHeaders time.Duration,
) *SpacedHeaderGenerator {
	return &SpacedHeaderGenerator{
		t:                  t,
		TimeBetweenHeaders: timeBetweenHeaders,
		currentTime:        startTime,
		currentHeight:      1,
	}
}

func (shg *SpacedHeaderGenerator) NextHeader() *header.ExtendedHeader {
	h := headertest.RandExtendedHeaderAtTimestamp(shg.t, shg.currentTime)
	h.RawHeader.Height = shg.currentHeight
	h.RawHeader.Time = shg.currentTime
	shg.currentHeight++
	shg.currentTime = shg.currentTime.Add(shg.TimeBetweenHeaders)
	return h
}
