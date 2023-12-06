package store

import (
	"context"
	"crypto/sha256"
	mrand "math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-app/pkg/da"
	"github.com/celestiaorg/celestia-app/pkg/wrapper"
	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds/edstest"
	"github.com/celestiaorg/celestia-node/share/sharetest"
)

func TestMemFileShare(t *testing.T) {
	eds := edstest.RandEDS(t, 32)
	root, err := share.NewRoot(eds)
	require.NoError(t, err)
	fl := &MemFile{Eds: eds}

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

func TestMemFileDate(t *testing.T) {
	size := 32

	// generate random shares
	shares := sharetest.RandShares(t, size*size)
	rand := mrand.New(mrand.NewSource(time.Now().UnixNano()))

	// choose random range in shares slice and set namespace to be the same for all shares in range
	from := rand.Intn(size * size)
	to := rand.Intn(size * size)
	if to < from {
		from, to = to, from
	}
	expected := shares[from]
	namespace := share.GetNamespace(expected)

	// change namespace for all shares in range
	for i := from; i <= to; i++ {
		shares[i] = expected
	}

	eds, err := rsmt2d.ComputeExtendedDataSquare(shares, share.DefaultRSMT2DCodec(), wrapper.NewConstructor(uint64(size)))
	require.NoError(t, err)
	dah, err := da.NewDataAvailabilityHeader(eds)
	require.NoError(t, err)

	file := &MemFile{Eds: eds}

	for i, root := range dah.RowRoots {
		if !namespace.IsOutsideRange(root, root) {
			nd, err := file.Data(context.Background(), namespace, i)
			require.NoError(t, err)
			ok := nd.Verify(root, namespace)
			require.True(t, ok)
		}
	}
}
