package store

import (
	"context"
	"crypto/sha256"
	mrand "math/rand"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds/edstest"
	"github.com/celestiaorg/celestia-node/share/sharetest"
)

type createFile func(eds *rsmt2d.ExtendedDataSquare) File

func testFileShare(t *testing.T, createFile createFile, size int) {
	eds := edstest.RandEDS(t, size)
	fl := createFile(eds)

	root, err := share.NewRoot(eds)
	require.NoError(t, err)

	width := int(eds.Width())
	for _, axisType := range []rsmt2d.Axis{rsmt2d.Col, rsmt2d.Row} {
		for i := 0; i < width*width; i++ {
			axisIdx, shrIdx := i/width, i%width
			if axisType == rsmt2d.Col {
				axisIdx, shrIdx = shrIdx, axisIdx
			}

			shr, prf, err := fl.Share(context.TODO(), axisType, axisIdx, shrIdx)
			require.NoError(t, err)

			namespace := share.ParitySharesNamespace
			if axisIdx < width/2 && shrIdx < width/2 {
				namespace = share.GetNamespace(shr)
			}

			axishash := root.RowRoots[axisIdx]
			if axisType == rsmt2d.Col {
				axishash = root.ColumnRoots[axisIdx]
			}

			ok := prf.VerifyInclusion(sha256.New(), namespace.ToNMT(), [][]byte{shr}, axishash)
			require.True(t, ok)
		}
	}
}

func testFileDate(t *testing.T, createFile createFile, size int) {
	// generate EDS with random data and some shares with the same namespace
	namespace := sharetest.RandV0Namespace()
	amount := mrand.Intn(size*size-1) + 1
	eds, dah := edstest.RandEDSWithNamespace(t, namespace, amount, size)

	f := createFile(eds)

	for i, root := range dah.RowRoots {
		if !namespace.IsOutsideRange(root, root) {
			nd, err := f.Data(context.Background(), namespace, i)
			require.NoError(t, err)
			ok := nd.Verify(root, namespace)
			require.True(t, ok)
		}
	}
}

func testFileAxisHalf(t *testing.T, createFile createFile, size int) {
	eds := edstest.RandEDS(t, size)
	fl := createFile(eds)

	for _, axisType := range []rsmt2d.Axis{rsmt2d.Col, rsmt2d.Row} {
		for i := 0; i < size; i++ {
			half, err := fl.AxisHalf(context.Background(), axisType, i)
			require.NoError(t, err)
			require.Equal(t, getAxis(eds, axisType, i)[:size], half)
		}
	}
}

func testFileEds(t *testing.T, createFile createFile, size int) {
	eds := edstest.RandEDS(t, size)
	fl := createFile(eds)

	eds2, err := fl.EDS(context.Background())
	require.NoError(t, err)
	require.True(t, eds.Equals(eds2))
}
