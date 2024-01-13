package store

import (
	"github.com/celestiaorg/rsmt2d"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestCacheFile(t *testing.T) {
	size := 8
	mem := newMemPools(NewCodec())
	newFile := func(eds *rsmt2d.ExtendedDataSquare) EdsFile {
		path := t.TempDir() + "/testfile"
		fl, err := CreateOdsFile(path, eds, mem)
		require.NoError(t, err)
		return NewCacheFile(fl, mem.codec)
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
}
