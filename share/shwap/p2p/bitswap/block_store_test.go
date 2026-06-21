package bitswap

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/share/eds"
)

// hasGetter is a configurable AccessorGetter used to drive Blockstore.Has.
type hasGetter struct {
	eds.AccessorStreamer
	has    bool
	hasErr error
}

func (g hasGetter) GetByHeight(context.Context, uint64) (eds.AccessorStreamer, error) {
	return g.AccessorStreamer, nil
}

func (g hasGetter) HasByHeight(context.Context, uint64) (bool, error) {
	return g.has, g.hasErr
}

// TestBlockstore_Has verifies that Has reflects the real result of
// HasByHeight instead of unconditionally reporting true. Reporting HAVE for a
// height the node does not store makes peers send a follow-up WANT-block that
// fails, delaying their failover to a peer that actually has the data.
func TestBlockstore_Has(t *testing.T) {
	ctx := context.Background()

	// a valid Shwap CID so Has reaches HasByHeight.
	blk, err := NewEmptyRowBlock(1, 0, 4)
	require.NoError(t, err)
	cid := blk.CID()

	t.Run("present", func(t *testing.T) {
		bs := &Blockstore{Getter: hasGetter{has: true}}
		has, err := bs.Has(ctx, cid)
		require.NoError(t, err)
		require.True(t, has)
	})

	t.Run("absent", func(t *testing.T) {
		bs := &Blockstore{Getter: hasGetter{has: false}}
		has, err := bs.Has(ctx, cid)
		require.NoError(t, err)
		require.False(t, has, "Has must report false when the height is not stored")
	})

	t.Run("error", func(t *testing.T) {
		bs := &Blockstore{Getter: hasGetter{hasErr: errors.New("boom")}}
		has, err := bs.Has(ctx, cid)
		require.Error(t, err)
		require.False(t, has)
	})
}
