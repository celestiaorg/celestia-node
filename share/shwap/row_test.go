package shwap

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds/edstest"
)

func TestRowFromShares(t *testing.T) {
	const odsSize = 8
	eds := edstest.RandEDS(t, odsSize)

	for rowIdx := 0; rowIdx < odsSize*2; rowIdx++ {
		for _, side := range []RowSide{Left, Right} {
			shares := eds.Row(uint(rowIdx))
			row := RowFromShares(shares, side)
			extended, err := row.Shares()
			require.NoError(t, err)
			require.Equal(t, shares, extended)

			var half []share.Share
			if side == Right {
				half = shares[odsSize:]
			} else {
				half = shares[:odsSize]
			}
			require.Equal(t, half, row.halfShares)
			require.Equal(t, side, row.side)
		}
	}
}

func TestRowValidate(t *testing.T) {
	const odsSize = 8
	eds := edstest.RandEDS(t, odsSize)
	root, err := share.NewRoot(eds)
	require.NoError(t, err)

	for rowIdx := 0; rowIdx < odsSize*2; rowIdx++ {
		for _, side := range []RowSide{Left, Right} {
			shares := eds.Row(uint(rowIdx))
			row := RowFromShares(shares, side)

			err := row.Validate(root, rowIdx)
			require.NoError(t, err)
			err = row.Validate(root, rowIdx)
			require.NoError(t, err)
		}
	}
}

func TestRowValidateNegativeCases(t *testing.T) {
	eds := edstest.RandEDS(t, 8) // Generate a random Extended Data Square of size 8
	root, err := share.NewRoot(eds)
	require.NoError(t, err)
	shares := eds.Row(0)
	row := RowFromShares(shares, Left)

	// Test with incorrect side specification
	invalidSideRow := Row{halfShares: row.halfShares, side: RowSide(999)}
	err = invalidSideRow.Validate(root, 0)
	require.Error(t, err, "should error on invalid row side")

	// Test with invalid shares (more shares than expected)
	incorrectShares := make([]share.Share, (eds.Width()/2)+1) // Adding an extra share
	for i := range incorrectShares {
		incorrectShares[i] = eds.GetCell(uint(i), 0)
	}
	invalidRow := Row{halfShares: incorrectShares, side: Left}
	err = invalidRow.Validate(root, 0)
	require.Error(t, err, "should error on incorrect number of shares")

	// Test with empty shares
	emptyRow := Row{halfShares: []share.Share{}, side: Left}
	err = emptyRow.Validate(root, 0)
	require.Error(t, err, "should error on empty halfShares")

	// Doesn't match root. Corrupt root hash
	root.RowRoots[0][len(root.RowRoots[0])-1] ^= 0xFF
	err = row.Validate(root, 0)
	require.Error(t, err, "should error on invalid root hash")
}

func TestRowProtoEncoding(t *testing.T) {
	const odsSize = 8
	eds := edstest.RandEDS(t, odsSize)

	for rowIdx := 0; rowIdx < odsSize*2; rowIdx++ {
		for _, side := range []RowSide{Left, Right} {
			shares := eds.Row(uint(rowIdx))
			row := RowFromShares(shares, side)

			pb := row.ToProto()
			rowOut := RowFromProto(pb)
			require.Equal(t, row, rowOut)
		}
	}
}

// BenchmarkRowValidate benchmarks the performance of row validation.
// BenchmarkRowValidate-10    	    9591	    121802 ns/op
func BenchmarkRowValidate(b *testing.B) {
	const odsSize = 32
	eds := edstest.RandEDS(b, odsSize)
	root, err := share.NewRoot(eds)
	require.NoError(b, err)
	shares := eds.Row(0)
	row := RowFromShares(shares, Left)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = row.Validate(root, 0)
	}
}
