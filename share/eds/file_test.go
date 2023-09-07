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
	eds := edstest.RandEDS(t, 16)

	fl, err := CreateFile(path, eds)
	require.NoError(t, err)
	err = fl.Close()
	require.NoError(t, err)

	fl, err = OpenFile(path)
	require.NoError(t, err)

	for i := 0; i < int(eds.Width()); i++ {
		row, err := fl.Axis(i, rsmt2d.Row)
		require.NoError(t, err)
		assert.EqualValues(t, eds.Row(uint(i)), row)
	}

	width := int(eds.Width())
	for i := 0; i < width*2; i++ {
		row, col := uint(i/width), uint(i%width)
		shr, err := fl.Share(i)
		require.NoError(t, err)
		assert.EqualValues(t, eds.GetCell(row, col), shr)

		shr, proof, err := fl.ShareWithProof(i, rsmt2d.Row)
		require.NoError(t, err)
		assert.EqualValues(t, eds.GetCell(row, col), shr)

		roots, err := eds.RowRoots()
		require.NoError(t, err)

		ok := proof.VerifyInclusion(sha256.New(), share.GetNamespace(shr).ToNMT(), [][]byte{shr}, roots[row])
		assert.True(t, ok)
	}

	out, err := fl.EDS()
	require.NoError(t, err)
	assert.True(t, eds.Equals(out))

	err = fl.Close()
	require.NoError(t, err)
}
