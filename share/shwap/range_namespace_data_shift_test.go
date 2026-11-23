package shwap_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	libshare "github.com/celestiaorg/go-square/v4/share"
	"github.com/celestiaorg/nmt"

	"github.com/celestiaorg/celestia-node/share/eds/edstest"
	"github.com/celestiaorg/celestia-node/share/shwap"
)

// TestRangeNamespaceDataRejectsMisboundProofs checks that verification rejects
// a response whose edge-row proofs do not cover exactly the requested range.
// Each case uses real shares and valid proofs from the square, cut so that the
// share count still matches the request.
func TestRangeNamespaceDataRejectsMisboundProofs(t *testing.T) {
	const odsSize = 4
	square, root := edstest.RandEDSWithNamespace(t, libshare.RandomNamespace(), odsSize*odsSize, odsSize)

	rowShares := func(row int) []libshare.Share {
		shrs, err := libshare.FromBytes(square.Row(uint(row)))
		require.NoError(t, err)
		return shrs
	}
	proof := func(row, fromCol, toCol int) *nmt.Proof {
		p, err := shwap.GenerateSharesProofs(row, fromCol, toCol, odsSize, rowShares(row))
		require.NoError(t, err)
		return p
	}
	ods := func(row, fromCol, toCol int) []libshare.Share {
		return rowShares(row)[fromCol:toCol]
	}

	tests := []struct {
		name     string
		from, to shwap.SampleCoords
		rngdata  shwap.RangeNamespaceData
	}{
		{
			// request [1, 11): row 0 is sent in full without a proof, and the
			// last row proof is moved to start at col 1. Index 0 is added and
			// index 8 is dropped.
			name: "missing first proof, shifted last proof",
			from: shwap.SampleCoords{Row: 0, Col: 1},
			to:   shwap.SampleCoords{Row: 2, Col: 2},
			rngdata: shwap.RangeNamespaceData{
				Shares:                 [][]libshare.Share{ods(0, 0, 4), ods(1, 0, 4), ods(2, 1, 3)},
				LastIncompleteRowProof: proof(2, 1, 3),
			},
		},
		{
			// request [1, 11): the first row proof ends at col 3 and row 2 is
			// sent in full without a proof. Index 3 is dropped and index 11
			// is added.
			name: "shortened first proof, missing last proof",
			from: shwap.SampleCoords{Row: 0, Col: 1},
			to:   shwap.SampleCoords{Row: 2, Col: 2},
			rngdata: shwap.RangeNamespaceData{
				Shares:                  [][]libshare.Share{ods(0, 1, 3), ods(1, 0, 4), ods(2, 0, 4)},
				FirstIncompleteRowProof: proof(0, 1, 3),
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.rngdata.VerifyInclusion(tc.from, tc.to, odsSize, root.RowRoots[tc.from.Row:tc.to.Row+1])
			require.Error(t, err)
		})
	}
}
