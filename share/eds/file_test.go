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

func TestCreateFile(t *testing.T) {
	path := t.TempDir() + "/testfile"
	edsIn := edstest.RandEDS(t, 8)

	for _, mode := range []FileMode{EDSMode, ODSMode} {
		f, err := CreateFile(path, edsIn, FileConfig{Mode: mode})
		require.NoError(t, err)
		edsOut, err := f.EDS()
		require.NoError(t, err)
		assert.True(t, edsIn.Equals(edsOut))
	}
}

func TestFile(t *testing.T) {
	path := t.TempDir() + "/testfile"
	eds := edstest.RandEDS(t, 8)
	root, err := share.NewRoot(eds)
	require.NoError(t, err)

	// TODO(@Wondartan): Test in multiple modes
	fl, err := CreateFile(path, eds)
	require.NoError(t, err)
	err = fl.Close()
	require.NoError(t, err)

	fl, err = OpenFile(path)
	require.NoError(t, err)

	axisTypes := []rsmt2d.Axis{rsmt2d.Col, rsmt2d.Row}
	for _, axisType := range axisTypes {
		for i := 0; i < int(eds.Width()); i++ {
			row, err := fl.Axis(axisType, i)
			require.NoError(t, err)
			assert.EqualValues(t, getAxis(axisType, i, eds), row)
		}
	}

	width := int(eds.Width())
	for _, axisType := range axisTypes {
		for i := 0; i < width*width; i++ {
			axisIdx, shrIdx := i/width, i%width
			if axisType == rsmt2d.Col {
				axisIdx, shrIdx = shrIdx, axisIdx
			}

			shr, prf, err := fl.ShareWithProof(axisType, axisIdx, shrIdx)
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
			assert.True(t, ok)
		}
	}

	out, err := fl.EDS()
	require.NoError(t, err)
	assert.True(t, eds.Equals(out))

	err = fl.Close()
	require.NoError(t, err)
}
