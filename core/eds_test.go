package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/types"

	"github.com/celestiaorg/celestia-app/pkg/da"

	"github.com/celestiaorg/celestia-node/share"
)

// TestTrulyEmptySquare ensures that a truly empty square (square size 1 and no
// txs) will be recognized as empty and return nil from `extendBlock` so that
// we do not redundantly store empty EDSes.
func TestTrulyEmptySquare(t *testing.T) {
	data := types.Data{
		Txs:        []types.Tx{},
		SquareSize: 1,
	}

	eds, err := extendBlock(data)
	require.NoError(t, err)
	assert.Nil(t, eds)
}

// TestNonEmptySquareWithZeroTxs tests that a non-empty square with no
// transactions or blobs computes the correct data root the minimum DAH.
// Technically, this block data is invalid because the construction of the
// square is deterministic, and the rules which dictate the square size do not
// allow for empty block data. However, should that ever occur, we need to to
// ensure that the correct data root is generated.
func TestNonEmptySquareWithZeroTxs(t *testing.T) {
	data := types.Data{
		Txs:        []types.Tx{},
		SquareSize: 16,
	}

	eds, err := extendBlock(data)
	require.NoError(t, err)
	dah := da.NewDataAvailabilityHeader(eds)
	assert.Equal(t, share.EmptyRoot().Hash(), dah.Hash())
}
