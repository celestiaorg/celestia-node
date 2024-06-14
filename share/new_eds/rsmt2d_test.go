package eds

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds/edstest"
	"github.com/celestiaorg/celestia-node/share/shwap"
)

func TestMemFile(t *testing.T) {
	odsSize := 8
	newAccessor := func(tb testing.TB, eds *rsmt2d.ExtendedDataSquare) Accessor {
		return &Rsmt2D{ExtendedDataSquare: eds}
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	t.Cleanup(cancel)

	TestSuiteAccessor(ctx, t, newAccessor, odsSize)
}

func TestRsmt2dHalfRow(t *testing.T) {
	const odsSize = 8
	eds, _ := randRsmt2dAccsessor(t, odsSize)

	for rowIdx := 0; rowIdx < odsSize*2; rowIdx++ {
		for _, side := range []shwap.RowSide{shwap.Left, shwap.Right} {
			row := eds.HalfRow(rowIdx, side)

			want := eds.Row(uint(rowIdx))
			shares, err := row.Shares()
			require.NoError(t, err)
			require.Equal(t, want, shares)
		}
	}
}

func TestRsmt2dSampleForProofAxis(t *testing.T) {
	const odsSize = 8
	eds := edstest.RandEDS(t, odsSize)
	accessor := Rsmt2D{ExtendedDataSquare: eds}

	for _, proofType := range []rsmt2d.Axis{rsmt2d.Row, rsmt2d.Col} {
		for rowIdx := 0; rowIdx < odsSize*2; rowIdx++ {
			for colIdx := 0; colIdx < odsSize*2; colIdx++ {
				sample, err := accessor.SampleForProofAxis(rowIdx, colIdx, proofType)
				require.NoError(t, err)

				want := eds.GetCell(uint(rowIdx), uint(colIdx))
				require.Equal(t, want, sample.Share)
				require.Equal(t, proofType, sample.ProofType)
				require.NotNil(t, sample.Proof)
				require.Equal(t, sample.Proof.End()-sample.Proof.Start(), 1)
				require.Len(t, sample.Proof.Nodes(), 4)
			}
		}
	}
}

func randRsmt2dAccsessor(t *testing.T, size int) (Rsmt2D, *share.Root) {
	eds := edstest.RandEDS(t, size)
	root, err := share.NewRoot(eds)
	require.NoError(t, err)
	return Rsmt2D{ExtendedDataSquare: eds}, root
}
