package core

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds/edstest"
	"github.com/celestiaorg/celestia-node/store"
)

// TestListener_DedupSkipsStoredHeight verifies the multi-source dedup gate in
// handleNewBlockHeight: a height already in the store (i.e. already processed)
// is skipped before the block is even fetched, so duplicate heights fanned in
// by MultiSource are dropped without re-downloading the block.
func TestListener_DedupSkipsStoredHeight(t *testing.T) {
	ctx := t.Context()

	st, err := store.NewStore(store.DefaultParameters(), t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, st.Stop(ctx)) })

	const height = uint64(42)
	eds := edstest.RandEDS(t, 4)
	roots, err := share.NewAxisRoots(eds)
	require.NoError(t, err)
	require.NoError(t, st.PutODSQ4(ctx, roots, height, eds))

	var fetched atomic.Bool
	cl := &Listener{
		store: st,
		fetcher: &fakeSource{getFn: func(context.Context, int64) (*SignedBlock, error) {
			fetched.Store(true)
			return nil, errors.New("must not fetch an already-stored height")
		}},
	}

	err = cl.handleNewBlockEvent(ctx, BlockEvent{Height: int64(height)})
	require.NoError(t, err)
	assert.False(t, fetched.Load(), "dedup gate should short-circuit before fetching the block")
}

// wrongChainSource serves a block stamped with the given chain ID at any
// height, for exercising the chainID guard behind the dedup gate.
func wrongChainSource(chainID string) *fakeSource {
	return &fakeSource{getFn: func(_ context.Context, h int64) (*SignedBlock, error) {
		b := blockAt(h)
		b.Header.ChainID = chainID
		return &b, nil
	}}
}

// TestListener_ChainIDCheckedAfterDedup verifies the chainID guard sits behind
// the dedup gate: a wrong-chain duplicate of an already-stored height is deduped
// away before it is even fetched (a slow misconfigured endpoint is tolerated),
// while a wrong-chain block for a fresh height — i.e. one whose fastest source
// is on the wrong network — is fetched and still panics.
func TestListener_ChainIDCheckedAfterDedup(t *testing.T) {
	ctx := t.Context()

	st, err := store.NewStore(store.DefaultParameters(), t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, st.Stop(ctx)) })

	const stored = uint64(42)
	eds := edstest.RandEDS(t, 4)
	roots, err := share.NewAxisRoots(eds)
	require.NoError(t, err)
	require.NoError(t, st.PutODSQ4(ctx, roots, stored, eds))

	cl := &Listener{store: st, fetcher: wrongChainSource("wrong-chain"), chainID: "expected-chain"}

	t.Run("wrong chain on a stored height is deduped, no panic", func(t *testing.T) {
		assert.NotPanics(t, func() {
			err := cl.handleNewBlockEvent(ctx, BlockEvent{Height: int64(stored)})
			require.NoError(t, err)
		})
	})

	t.Run("wrong chain on a fresh height panics", func(t *testing.T) {
		assert.Panics(t, func() {
			_ = cl.handleNewBlockEvent(ctx, BlockEvent{Height: int64(stored) + 1})
		})
	})
}

// blockAtTime serves a block stamped with the given time at any height, for
// exercising the availability-window gate.
func blockAtTime(ts time.Time) *fakeSource {
	return &fakeSource{getFn: func(_ context.Context, h int64) (*SignedBlock, error) {
		b := blockAt(h)
		b.Header.Time = ts
		return &b, nil
	}}
}

// TestListener_HistoricBlockSkippedAfterFetch verifies the availability-window
// gate sits right after the fetch: a non-archival listener drops a block older
// than the window before querying sync state (and so before any EDS
// construction or storage), while a within-window block proceeds past the gate.
// This is what keeps a source replaying history during catch-up from costing an
// erasure-coding pass per replayed height.
func TestListener_HistoricBlockSkippedAfterFetch(t *testing.T) {
	ctx := t.Context()

	st, err := store.NewStore(store.DefaultParameters(), t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, st.Stop(ctx)) })

	const window = time.Hour
	// syncingErr poisons IsSyncingFrom: reaching the sync-status query at all
	// fails the historic case and is the observable proof that a within-window
	// block passed the gate.
	src := blockAtTime(time.Now().Add(-2 * window))
	src.syncingErr = errors.New("sync state must not be queried for a historic block")

	cl := &Listener{store: st, fetcher: src, availabilityWindow: window}

	t.Run("historic block is dropped after fetch, before sync-status query", func(t *testing.T) {
		err := cl.handleNewBlockEvent(ctx, BlockEvent{Height: 7})
		require.NoError(t, err)

		has, err := st.HasByHeight(ctx, 7)
		require.NoError(t, err)
		assert.False(t, has, "historic block must not be stored")
	})

	t.Run("within-window block passes the gate", func(t *testing.T) {
		fresh := blockAtTime(time.Now())
		fresh.syncingErr = errors.New("queried")
		cl := &Listener{store: st, fetcher: fresh, availabilityWindow: window}

		err := cl.handleNewBlockEvent(ctx, BlockEvent{Height: 8})
		require.ErrorContains(t, err, "queried", "a fresh block must reach the sync-status query")
	})

	t.Run("archival listener keeps historic blocks flowing", func(t *testing.T) {
		archival := blockAtTime(time.Now().Add(-2 * window))
		archival.syncingErr = errors.New("queried")
		cl := &Listener{store: st, fetcher: archival, availabilityWindow: window, archival: true}

		err := cl.handleNewBlockEvent(ctx, BlockEvent{Height: 9})
		require.ErrorContains(t, err, "queried", "an archival node must process historic blocks")
	})
}
