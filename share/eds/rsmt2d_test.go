package eds

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	libshare "github.com/celestiaorg/go-square/v3/share"
	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds/edstest"
	"github.com/celestiaorg/celestia-node/share/shwap"
)

func TestRsmt2dAccessor(t *testing.T) {
	odsSize := 16
	newAccessor := func(tb testing.TB, eds *rsmt2d.ExtendedDataSquare) Accessor {
		return &Rsmt2D{ExtendedDataSquare: eds}
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*15)
	t.Cleanup(cancel)

	TestSuiteAccessor(ctx, t, newAccessor, odsSize)

	newStreamer := func(tb testing.TB, eds *rsmt2d.ExtendedDataSquare) AccessorStreamer {
		return &Rsmt2D{ExtendedDataSquare: eds}
	}
	TestStreamer(ctx, t, newStreamer, odsSize)
}

func TestRsmt2dHalfRow(t *testing.T) {
	const odsSize = 8
	eds, _ := randRsmt2dAccsessor(t, odsSize)

	for rowIdx := range odsSize * 2 {
		for _, side := range []shwap.RowSide{shwap.Left, shwap.Right} {
			row, err := eds.HalfRow(rowIdx, side)
			require.NoError(t, err)

			want := eds.Row(uint(rowIdx))
			shares, err := row.Shares()
			require.NoError(t, err)
			require.Equal(t, want, libshare.ToBytes(shares))
		}
	}
}

func TestRsmt2dSampleForProofAxis(t *testing.T) {
	const odsSize = 8
	eds := edstest.RandEDS(t, odsSize)
	accessor := Rsmt2D{ExtendedDataSquare: eds}

	for _, proofType := range []rsmt2d.Axis{rsmt2d.Row, rsmt2d.Col} {
		for rowIdx := range odsSize * 2 {
			for colIdx := range odsSize * 2 {
				idx := shwap.SampleCoords{Row: rowIdx, Col: colIdx}

				sample, err := accessor.SampleForProofAxis(idx, proofType)
				require.NoError(t, err)

				want := eds.GetCell(uint(rowIdx), uint(colIdx))
				require.Equal(t, want, sample.ToBytes())
				require.Equal(t, proofType, sample.ProofType)
				require.NotNil(t, sample.Proof)
				require.Equal(t, sample.Proof.End()-sample.Proof.Start(), 1)
				require.Len(t, sample.Proof.Nodes(), 4)
			}
		}
	}
}

func randRsmt2dAccsessor(t *testing.T, size int) (Rsmt2D, *share.AxisRoots) {
	eds := edstest.RandEDS(t, size)
	root, err := share.NewAxisRoots(eds)
	require.NoError(t, err)
	return Rsmt2D{ExtendedDataSquare: eds}, root
}
