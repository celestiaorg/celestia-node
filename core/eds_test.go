package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/types"

	"github.com/celestiaorg/celestia-app/app"
	"github.com/celestiaorg/celestia-app/pkg/appconsts"
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

	eds, err := extendBlock(data, appconsts.LatestVersion)
	require.NoError(t, err)
	assert.Nil(t, eds)
}

// TestNonZeroSquareSize tests that the DAH hash of a block with no transactions
// is equal to the DAH hash for an empty root even if SquareSize is set to
// something non-zero. Technically, this block data is invalid because the
// construction of the square is deterministic, and the rules which dictate the
// square size do not allow for empty block data. However, should that ever
// occur, we need to ensure that the correct data root is generated.
func TestEmptySquareWithZeroTxs(t *testing.T) {
	data := types.Data{
		Txs: []types.Tx{},
	}

	eds, err := extendBlock(data, appconsts.LatestVersion)
	require.Nil(t, eds)
	require.NoError(t, err)

	// force extend the square using an empty block and compare with the min DAH
	eds, err = app.ExtendBlock(data, appconsts.LatestVersion)
	require.NoError(t, err)

	dah, err := da.NewDataAvailabilityHeader(eds)
	require.NoError(t, err)
	assert.Equal(t, share.EmptyRoot().Hash(), dah.Hash())
}
