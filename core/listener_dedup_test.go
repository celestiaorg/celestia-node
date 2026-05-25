package core

import (
	"errors"
	"sync/atomic"
	"testing"

	"github.com/cometbft/cometbft/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds/edstest"
	"github.com/celestiaorg/celestia-node/store"
)

// TestListener_DedupSkipsStoredHeight verifies the multi-source dedup gate in
// handleNewSignedBlock: a block whose height is already in the store (i.e.
// already processed) is skipped before any work, so duplicate copies fanned in
// by MultiSource are dropped cheaply.
//
// The skip path is self-validating: blockAt has a nil Data, so if the gate did
// NOT short-circuit, the subsequent da.ConstructEDS(b.Data.Txs...) would panic
// and construct would run. A clean nil return with construct untouched can only
// happen if the gate fired.
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

	var constructed atomic.Bool
	cl := &Listener{
		store: st,
		construct: func(
			*types.Header,
			*types.Commit,
			*types.ValidatorSet,
			*rsmt2d.ExtendedDataSquare,
		) (*header.ExtendedHeader, error) {
			constructed.Store(true)
			return nil, errors.New("construct must not run for an already-stored height")
		},
	}

	err = cl.handleNewSignedBlock(ctx, blockAt(int64(height)))
	require.NoError(t, err)
	assert.False(t, constructed.Load(), "dedup gate should short-circuit before construct")
}

// blockAtWithChainID is blockAt with an explicit chain ID, for exercising the
// chainID guard.
func blockAtWithChainID(h int64, chainID string) SignedBlock {
	b := blockAt(h)
	b.Header.ChainID = chainID
	return b
}

// TestListener_ChainIDCheckedAfterDedup verifies the chainID guard sits behind
// the dedup gate: a wrong-chain duplicate of an already-stored height is deduped
// away without panicking (a slow misconfigured endpoint is tolerated), while a
// wrong-chain block for a fresh height — i.e. one whose fastest source is on the
// wrong network — still panics.
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

	cl := &Listener{store: st, chainID: "expected-chain"}

	t.Run("wrong chain on a stored height is deduped, no panic", func(t *testing.T) {
		assert.NotPanics(t, func() {
			err := cl.handleNewSignedBlock(ctx, blockAtWithChainID(int64(stored), "wrong-chain"))
			require.NoError(t, err)
		})
	})

	t.Run("wrong chain on a fresh height panics", func(t *testing.T) {
		assert.Panics(t, func() {
			_ = cl.handleNewSignedBlock(ctx, blockAtWithChainID(int64(stored)+1, "wrong-chain"))
		})
	})
}
