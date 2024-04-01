package file

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/rsmt2d"
)

func TestQ1Q4File(t *testing.T) {
	size := 8
	createOdsFile := func(eds *rsmt2d.ExtendedDataSquare) EdsFile {
		path := t.TempDir() + "/testfile"
		fl, err := CreateQ1Q4File(path, []byte{}, eds)
		require.NoError(t, err)
		return fl
	}

	t.Run("Share", func(t *testing.T) {
		testFileShare(t, createOdsFile, size)
	})

	t.Run("AxisHalf", func(t *testing.T) {
		testFileAxisHalf(t, createOdsFile, size)
	})

	t.Run("Data", func(t *testing.T) {
		testFileData(t, createOdsFile, size)
	})

	t.Run("EDS", func(t *testing.T) {
		testFileEds(t, createOdsFile, size)
	})

	t.Run("ReadOds", func(t *testing.T) {
		testFileReader(t, createOdsFile, size)
	})
}
