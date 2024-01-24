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

	hdr "github.com/celestiaorg/go-header"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/header/headertest"
)

/*
	| toPrune  | availability window |
*/

func TestServiceCheckpointing(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	store := headertest.NewStore(t)

	mp := &mockPruner{}

	serv := NewService(
		mp,
		AvailabilityWindow(time.Second),
		store,
		sync.MutexWrap(datastore.NewMapDatastore()),
		time.Millisecond,
		WithGCCycle(0), // we do not need to run GC in this test
	)

	err := serv.Start(ctx)
	require.NoError(t, err)

	gen, err := store.GetByHeight(ctx, 1)
	require.NoError(t, err)

	// ensure checkpoint was initialized correctly
	assert.Equal(t, uint64(1), serv.checkpoint.LastPrunedHeight)
	assert.Empty(t, serv.checkpoint.FailedHeaders)
	assert.Equal(t, gen, serv.lastPruned())

	// update checkpoint
	lastPruned, err := store.GetByHeight(ctx, 3)
	require.NoError(t, err)
	err = serv.updateCheckpoint(ctx, lastPruned, map[uint64]error{2: fmt.Errorf("failed to prune")})
	require.NoError(t, err)

	err = serv.Stop(ctx)
	require.NoError(t, err)

	// ensure checkpoint was updated correctly in datastore
	err = serv.loadCheckpoint(ctx)
	require.NoError(t, err)
	assert.Equal(t, uint64(3), serv.checkpoint.LastPrunedHeight)
	assert.Len(t, serv.checkpoint.FailedHeaders, 1)
	assert.Equal(t, lastPruned, serv.lastPruned())

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
			expectedLength: 24 * 7,
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
			expectedLength: 2 * 24 * 7,
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

			serv := NewService(
				mp,
				tc.availWindow,
				store,
				sync.MutexWrap(datastore.NewMapDatastore()),
				tc.blockTime,
			)

			err := serv.Start(ctx)
			require.NoError(t, err)

			pruneable, err := serv.findPruneableHeaders(ctx)
			require.NoError(t, err)
			require.Len(t, pruneable, tc.expectedLength)

			pruneableCutoff := time.Now().Add(-time.Duration(tc.availWindow))
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
	deletedHeaderHashes []hdr.Hash
}

func (mp *mockPruner) Prune(_ context.Context, h *header.ExtendedHeader) error {
	mp.deletedHeaderHashes = append(mp.deletedHeaderHashes, h.Hash())
	return nil
}

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
