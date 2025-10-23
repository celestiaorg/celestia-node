package core

import (
	"testing"

	"github.com/cometbft/cometbft/types"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-app/v6/pkg/appconsts"
	"github.com/celestiaorg/celestia-app/v6/pkg/da"
	libshare "github.com/celestiaorg/go-square/v3/share"

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

	eds, err := da.ConstructEDS(data.Txs.ToSliceOfBytes(), appconsts.Version, -1)
	require.NoError(t, err)
	require.True(t, eds.Equals(share.EmptyEDS()))
}

// TestEmptySquareWithZeroTxs tests that the datahash of a block with no transactions
// is equal to the datahash of an empty eds, even if SquareSize is set to
// something non-zero. Technically, this block data is invalid because the
// construction of the square is deterministic, and the rules which dictate the
// square size do not allow for empty block data. However, should that ever
// occur, we need to ensure that the correct data root is generated.
func TestEmptySquareWithZeroTxs(t *testing.T) {
	data := types.Data{
		Txs: []types.Tx{},
	}

	eds, err := da.ConstructEDS(data.Txs.ToSliceOfBytes(), appconsts.Version, -1)
	require.NoError(t, err)
	require.True(t, eds.Equals(share.EmptyEDS()))

	// create empty shares and extend them manually
	emptyShares := libshare.TailPaddingShares(libshare.MinShareCount)
	rawEmptyShares := libshare.ToBytes(emptyShares)

	// extend the empty shares
	manualEds, err := da.ExtendShares(rawEmptyShares)
	require.NoError(t, err)

	// verify the manually extended EDS equals the empty EDS
	require.True(t, manualEds.Equals(share.EmptyEDS()))

	// verify the roots hash matches the empty EDS roots hash
	manualRoots, err := share.NewAxisRoots(manualEds)
	require.NoError(t, err)
	require.Equal(t, share.EmptyEDSRoots().Hash(), manualRoots.Hash())
}
