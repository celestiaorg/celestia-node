package file

import (
	"github.com/celestiaorg/rsmt2d"
	"testing"
)

func TestMemFile(t *testing.T) {
	size := 8
	newFile := func(eds *rsmt2d.ExtendedDataSquare) EdsFile {
		return &MemFile{Eds: eds}
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
