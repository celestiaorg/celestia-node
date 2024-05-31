package eds

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds/edstest"
	"github.com/celestiaorg/celestia-node/share/sharetest"
	"github.com/celestiaorg/celestia-node/share/shwap"
)

func TestMemFileSample(t *testing.T) {
	eds, root := randInMemEDS(t, 8)

	width := int(eds.Width())
	for rowIdx := 0; rowIdx < width; rowIdx++ {
		for colIdx := 0; colIdx < width; colIdx++ {
			shr, err := eds.Sample(context.TODO(), rowIdx, colIdx)
			require.NoError(t, err)

			err = shr.Validate(root, rowIdx, colIdx)
			require.NoError(t, err)
		}
	}
}

func TestHalfRowFromInMem(t *testing.T) {
	const odsSize = 8
	eds, _ := randInMemEDS(t, odsSize)

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

func TestInMemNamespacedData(t *testing.T) {
	const odsSize = 8

	sharesAmount := odsSize * odsSize
	namespace := sharetest.RandV0Namespace()
	for amount := 1; amount < sharesAmount; amount++ {
		eds, root := edstest.RandEDSWithNamespace(t, namespace, amount, odsSize)
		inMem := InMem{ExtendedDataSquare: eds}
		nd, err := inMem.NamespacedData(namespace)
		require.NoError(t, err)
		require.True(t, len(nd) > 0)
		require.Len(t, nd.Flatten(), amount)

		err = nd.Validate(root, namespace)
		require.NoError(t, err)
	}
}

func TestInMemSampleForProofAxis(t *testing.T) {
	const odsSize = 8
	eds := edstest.RandEDS(t, odsSize)
	inMem := InMem{ExtendedDataSquare: eds}

	for _, proofType := range []rsmt2d.Axis{rsmt2d.Row, rsmt2d.Col} {
		for rowIdx := 0; rowIdx < odsSize*2; rowIdx++ {
			for colIdx := 0; colIdx < odsSize*2; colIdx++ {
				sample, err := inMem.SampleForProofAxis(rowIdx, colIdx, proofType)
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

func randInMemEDS(t *testing.T, size int) (InMem, *share.Root) {
	eds := edstest.RandEDS(t, size)
	root, err := share.NewRoot(eds)
	require.NoError(t, err)
	return InMem{ExtendedDataSquare: eds}, root
}
