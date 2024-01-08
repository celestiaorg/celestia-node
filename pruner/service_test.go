package pruner

import (
	"context"
	"testing"
	"time"

	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/sync"
	"github.com/stretchr/testify/require"

	hdr "github.com/celestiaorg/go-header"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/header/headertest"
)

/*
	| toPrune  | availability window |
*/

// TODO @renaynay: tweak/document
var (
	availWindow = AvailabilityWindow(time.Millisecond)
	blockTime   = time.Millisecond * 100
	gcCycle     = time.Millisecond * 500
)

func TestService(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	store := headertest.NewStore(t)

	mp := &mockPruner{}

	serv := NewService(
		mp,
		availWindow,
		store,
		sync.MutexWrap(datastore.NewMapDatastore()),
		blockTime,
		WithGCCycle(gcCycle),
	)

	gen, err := store.GetByHeight(ctx, 1)
	require.NoError(t, err)

	err = serv.updateCheckpoint(ctx, gen)
	require.NoError(t, err)

	err = serv.Start(ctx)
	require.NoError(t, err)

	time.Sleep(time.Second)

	err = serv.Stop(ctx)
	require.NoError(t, err)

	t.Log(len(mp.deletedHeaderHashes)) // TODO @renaynay: expect something here
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
