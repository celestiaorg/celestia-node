package shwap

import (
	"testing"

	"github.com/stretchr/testify/require"

	libshare "github.com/celestiaorg/go-square/v2/share"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds/edstest"
)

func TestRowShares(t *testing.T) {
	const odsSize = 8
	eds := edstest.RandEDS(t, odsSize)

	for rowIdx := 0; rowIdx < odsSize*2; rowIdx++ {
		for _, side := range []RowSide{Left, Right, Both} {
			row, err := RowFromEDS(eds, rowIdx, side)
			require.NoError(t, err)
			require.Equal(t, side, row.side)

			extended, err := row.Shares()
			require.NoError(t, err)
			require.Len(t, extended, odsSize*2)
			require.Equal(t, Both, row.side)
		}
	}
}

func TestRowValidate(t *testing.T) {
	const odsSize = 8
	eds := edstest.RandEDS(t, odsSize)
	root, err := share.NewAxisRoots(eds)
	require.NoError(t, err)

	for rowIdx := 0; rowIdx < odsSize*2; rowIdx++ {
		for _, side := range []RowSide{Left, Right, Both} {
			row, err := RowFromEDS(eds, rowIdx, side)
			require.NoError(t, err)

			err = row.Verify(root, rowIdx)
			require.NoError(t, err)
			err = row.Verify(root, rowIdx)
			require.NoError(t, err)
		}
	}
}

func TestRowValidateNegativeCases(t *testing.T) {
	eds := edstest.RandEDS(t, 8) // Generate a random Extended Data Square of size 8
	root, err := share.NewAxisRoots(eds)
	require.NoError(t, err)
	shrs := eds.Row(0)
	shares, err := libshare.FromBytes(shrs)
	require.NoError(t, err)
	row := NewRow(shares, Left)

	// Test with incorrect side specification
	invalidSideRow := Row{shares: row.shares, side: RowSide(999)}
	err = invalidSideRow.Verify(root, 0)
	require.Error(t, err, "should error on invalid row side")

	// Test with invalid shares (more shares than expected)
	incorrectShares := make([]libshare.Share, (eds.Width()/2)+1) // Adding an extra share
	for i := range incorrectShares {
		shr, err := libshare.NewShare(eds.GetCell(uint(i), 0))
		require.NoError(t, err)
		incorrectShares[i] = *shr
	}
	invalidRow := Row{shares: incorrectShares, side: Left}
	err = invalidRow.Verify(root, 0)
	require.Error(t, err, "should error on incorrect number of shares")

	// Test with empty shares
	emptyRow := Row{shares: []libshare.Share{}, side: Left}
	err = emptyRow.Verify(root, 0)
	require.Error(t, err, "should error on empty halfShares")

	// Doesn't match root. Corrupt root hash
	root.RowRoots[0][len(root.RowRoots[0])-1] ^= 0xFF
	err = row.Verify(root, 0)
	require.Error(t, err, "should error on invalid root hash")
}

func TestRowProtoEncoding(t *testing.T) {
	const odsSize = 8
	eds := edstest.RandEDS(t, odsSize)

	for rowIdx := 0; rowIdx < odsSize*2; rowIdx++ {
		for _, side := range []RowSide{Left, Right, Both} {
			row, err := RowFromEDS(eds, rowIdx, side)
			require.NoError(t, err)

			pb := row.ToProto()
			rowOut, err := RowFromProto(pb)
			require.NoError(t, err)
			if side == Both {
				require.NotEqual(t, row, rowOut)
			} else {
				require.Equal(t, row, rowOut)
			}
		}
	}
}

// BenchmarkRowValidate benchmarks the performance of row validation.
// BenchmarkRowValidate-10    	    9591	    121802 ns/op
func BenchmarkRowValidate(b *testing.B) {
	const odsSize = 32
	eds := edstest.RandEDS(b, odsSize)
	root, err := share.NewAxisRoots(eds)
	require.NoError(b, err)
	row, err := RowFromEDS(eds, 0, Left)
	require.NoError(b, err)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = row.Verify(root, 0)
	}
}
