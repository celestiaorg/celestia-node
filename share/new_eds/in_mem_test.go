package eds

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
	fl := &InMem{ExtendedDataSquare: eds}

	width := int(eds.Width())
	for rowIdx := 0; rowIdx < width; rowIdx++ {
		for colIdx := 0; colIdx < width; colIdx++ {
			shr, err := fl.Share(context.TODO(), rowIdx, colIdx)
			require.NoError(t, err)

			err = shr.Validate(root, rowIdx, colIdx)
			require.NoError(t, err)
		}
	}
}

func TestMemFileDate(t *testing.T) {
	size := 32

	// generate InMem with random data and some shares with the same namespace
	namespace := sharetest.RandV0Namespace()
	amount := mrand.Intn(size*size-1) + 1
	eds, dah := edstest.RandEDSWithNamespace(t, namespace, amount, size)

	file := &InMem{ExtendedDataSquare: eds}

	for i, root := range dah.RowRoots {
		if !namespace.IsOutsideRange(root, root) {
			nd, err := file.Data(context.Background(), namespace, i)
			require.NoError(t, err)
			err = nd.Validate(dah, namespace, i)
			require.NoError(t, err)
		}
	}
}
