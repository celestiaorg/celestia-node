package store

import (
	"context"
	mrand "math/rand"
	"testing"

	"github.com/stretchr/testify/require"

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
	for _, proofType := range []ProofAxis{ProofAxisAny, ProofAxisRow, ProofAxisCol} {
		for x := 0; x < width; x++ {
			for y := 0; y < width; y++ {
				shr, err := fl.Share(context.TODO(), x, y, proofType)
				require.NoError(t, err)

				axishash := root.RowRoots[y]
				if proofType == ProofAxisCol {
					axishash = root.ColumnRoots[x]
				}

				ok := shr.Validate(axishash, x, y, width)
				require.True(t, ok)
			}
		}
	}
}

func TestMemFileDate(t *testing.T) {
	size := 32

	// generate EDS with random data and some shares with the same namespace
	namespace := sharetest.RandV0Namespace()
	amount := mrand.Intn(size*size-1) + 1
	eds, dah := edstest.RandEDSWithNamespace(t, namespace, amount, size)

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
