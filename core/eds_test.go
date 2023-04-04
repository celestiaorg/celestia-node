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
		Blobs:      []types.Blob{},
		SquareSize: 1,
	}

	eds, err := extendBlock(data)
	require.NoError(t, err)
	assert.Nil(t, eds)
}

// TestNonEmptySquareWithZeroTxs tests that a non-empty square with no
// transactions or blobs computes the correct data root (not the minimum DAH).
func TestNonEmptySquareWithZeroTxs(t *testing.T) {
	data := types.Data{
		Txs:        []types.Tx{},
		Blobs:      []types.Blob{},
		SquareSize: 16,
	}

	eds, err := extendBlock(data)
	require.NoError(t, err)
	dah := da.NewDataAvailabilityHeader(eds)
	assert.NotEqual(t, share.EmptyRoot().Hash(), dah.Hash())
}
