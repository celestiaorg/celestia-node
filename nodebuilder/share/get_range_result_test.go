package share

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-app/v10/pkg/da"
	libshare "github.com/celestiaorg/go-square/v4/share"

	"github.com/celestiaorg/celestia-node/share/eds"
	"github.com/celestiaorg/celestia-node/share/eds/edstest"
)

const testODSSize = 4

// newTestRangeResult builds a GetRangeResult for [start, end) of a square
// filled with one namespace and returns it with the square's data root.
func newTestRangeResult(t *testing.T, start, end int) (*GetRangeResult, []byte) {
	t.Helper()
	square, _ := edstest.RandEDSWithNamespace(
		t, libshare.RandomNamespace(), testODSSize*testODSSize, testODSSize,
	)
	dah, err := da.NewDataAvailabilityHeader(square)
	require.NoError(t, err)
	rngData, err := (&eds.Rsmt2D{ExtendedDataSquare: square}).RangeNamespaceData(context.Background(), start, end)
	require.NoError(t, err)
	res, err := newGetRangeResult(start, end, &rngData, &dah)
	require.NoError(t, err)
	return res, dah.Hash()
}

func TestGetRangeResultVerify(t *testing.T) {
	testCases := []struct {
		name    string
		mutate  func(*GetRangeResult)
		wantErr bool
	}{
		{
			name:    "valid",
			mutate:  func(*GetRangeResult) {},
			wantErr: false,
		},
		{
			name:    "nil proof",
			mutate:  func(r *GetRangeResult) { r.Proof = nil },
			wantErr: true,
		},
		{
			name:    "fewer proof shares than shares",
			mutate:  func(r *GetRangeResult) { r.Proof.Data = r.Proof.Data[:1] },
			wantErr: true,
		},
		{
			name:    "more proof shares than shares",
			mutate:  func(r *GetRangeResult) { r.Shares = r.Shares[:1] },
			wantErr: true,
		},
		{
			name: "share namespace differs from proof namespace",
			mutate: func(r *GetRangeResult) {
				r.Proof.NamespaceID = libshare.RandomNamespace().ID()
			},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// A range spanning three rows, starting and ending mid-row.
			res, root := newTestRangeResult(t, 2, 11)
			tc.mutate(res)
			err := res.Verify(root)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestGetRangeResultVerifyRange(t *testing.T) {
	res, root := newTestRangeResult(t, 2, 11)
	require.NoError(t, res.VerifyRange(root, 2, 11))
	require.Error(t, res.VerifyRange(root, 3, 12), "proof for [2, 11) must not verify as [3, 12)")
	require.Error(t, res.VerifyRange(root, 2, 10), "proof for [2, 11) must not verify as [2, 10)")

	// A single-row range ending at the row boundary.
	res, root = newTestRangeResult(t, 4, 8)
	require.NoError(t, res.VerifyRange(root, 4, 8))
	require.Error(t, res.VerifyRange(root, 0, 4))
}
