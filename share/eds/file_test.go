package eds

import (
	"crypto/sha256"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds/edstest"
)

func TestFile(t *testing.T) {
	path := t.TempDir() + "/testfile"
	eds := edstest.RandEDS(t, 8)
	root, err := share.NewRoot(eds)
	require.NoError(t, err)

	fl, err := CreateFile(path, eds)
	require.NoError(t, err)
	err = fl.Close()
	require.NoError(t, err)

	fl, err = OpenFile(path)
	require.NoError(t, err)

	axis := []rsmt2d.Axis{rsmt2d.Col, rsmt2d.Row}
	for _, axis := range axis {
		for i := 0; i < int(eds.Width()); i++ {
			row, err := fl.Axis(i, axis)
			require.NoError(t, err)
			assert.EqualValues(t, getAxis(i, axis, eds), row)
		}
	}

	width := int(eds.Width())
	for _, axis := range axis {
		for i := 0; i < width*width; i++ {
			row, col := uint(i/width), uint(i%width)
			shr, err := fl.Share(i)
			require.NoError(t, err)
			assert.EqualValues(t, eds.GetCell(row, col), shr)

			shr, proof, err := fl.ShareWithProof(i, axis)
			require.NoError(t, err)
			assert.EqualValues(t, eds.GetCell(row, col), shr)

			namespace := share.ParitySharesNamespace
			if int(row) < width/2 && int(col) < width/2 {
				namespace = share.GetNamespace(shr)
			}

			dahroot := root.RowRoots[row]
			if axis == rsmt2d.Col {
				dahroot = root.ColumnRoots[col]
			}

			ok := proof.VerifyInclusion(sha256.New(), namespace.ToNMT(), [][]byte{shr}, dahroot)
			assert.True(t, ok)
		}
	}

	out, err := fl.EDS()
	require.NoError(t, err)
	assert.True(t, eds.Equals(out))

	err = fl.Close()
	require.NoError(t, err)
}

// TODO(@Wondertan): Should be a method on eds
func getAxis(idx int, axis rsmt2d.Axis, eds *rsmt2d.ExtendedDataSquare) [][]byte {
	switch axis {
	case rsmt2d.Row:
		return eds.Row(uint(idx))
	case rsmt2d.Col:
		return eds.Col(uint(idx))
	default:
		panic("")
	}
}
