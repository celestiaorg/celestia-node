package file

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/rsmt2d"
)

func TestCacheFile(t *testing.T) {
	size := 8
	newFile := func(eds *rsmt2d.ExtendedDataSquare) EdsFile {
		path := t.TempDir() + "/testfile"
		fl, err := CreateOdsFile(path, []byte{}, eds)
		require.NoError(t, err)
		return NewCacheFile(fl)
	}

	t.Run("Share", func(t *testing.T) {
		testFileShare(t, newFile, size)
	})

	t.Run("AxisHalf", func(t *testing.T) {
		testFileAxisHalf(t, newFile, size)
	})

	t.Run("Data", func(t *testing.T) {
		testFileData(t, newFile, size)
	})

	t.Run("EDS", func(t *testing.T) {
		testFileEds(t, newFile, size)
	})

	t.Run("ReadOds", func(t *testing.T) {
		testFileReader(t, newFile, size)
	})
}
