package store

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/share/eds/edstest"
)

func TestCreateEdsFile(t *testing.T) {
	path := t.TempDir() + "/testfile"
	edsIn := edstest.RandEDS(t, 8)

	_, err := CreateEdsFile(path, edsIn)
	require.NoError(t, err)

	f, err := OpenEdsFile(path)
	require.NoError(t, err)
	edsOut, err := f.EDS(context.TODO())
	require.NoError(t, err)
	assert.True(t, edsIn.Equals(edsOut))
}

func TestEdsFile(t *testing.T) {
	size := 32
	createOdsFile := func(eds *rsmt2d.ExtendedDataSquare) File {
		path := t.TempDir() + "/testfile"
		fl, err := CreateEdsFile(path, eds)
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
		testFileDate(t, createOdsFile, size)
	})

	t.Run("EDS", func(t *testing.T) {
		testFileEds(t, createOdsFile, size)
	})
}

// BenchmarkAxisFromEdsFile/Size:32/Axis:row/squareHalf:first(original)-10         	  288624	      3758 ns/op
// BenchmarkAxisFromEdsFile/Size:32/Axis:row/squareHalf:second(extended)-10        	  313893	      3729 ns/op
// BenchmarkAxisFromEdsFile/Size:32/Axis:col/squareHalf:first(original)-10         	   29406	     41051 ns/op
// BenchmarkAxisFromEdsFile/Size:32/Axis:col/squareHalf:second(extended)-10        	   29145	     41047 ns/op
// BenchmarkAxisFromEdsFile/Size:64/Axis:row/squareHalf:first(original)-10         	  186302	      6532 ns/op
// BenchmarkAxisFromEdsFile/Size:64/Axis:row/squareHalf:second(extended)-10        	  186172	      6383 ns/op
// BenchmarkAxisFromEdsFile/Size:64/Axis:col/squareHalf:first(original)-10         	   14451	     82114 ns/op
// BenchmarkAxisFromEdsFile/Size:64/Axis:col/squareHalf:second(extended)-10        	   14572	     82047 ns/op
// BenchmarkAxisFromEdsFile/Size:128/Axis:row/squareHalf:first(original)-10        	   94576	     11349 ns/op
// BenchmarkAxisFromEdsFile/Size:128/Axis:row/squareHalf:second(extended)-10       	  103954	     11276 ns/op
// BenchmarkAxisFromEdsFile/Size:128/Axis:col/squareHalf:first(original)-10        	    7072	    165301 ns/op
// BenchmarkAxisFromEdsFile/Size:128/Axis:col/squareHalf:second(extended)-10       	    6805	    165173 ns/op
func BenchmarkAxisFromEdsFile(b *testing.B) {
	minSize, maxSize := 32, 128
	dir := b.TempDir()
	newFile := func(size int) File {
		eds := edstest.RandEDS(b, size)
		path := dir + "/testfile"
		f, err := CreateEdsFile(path, eds)
		require.NoError(b, err)
		return f
	}
	benchGetAxisFromFile(b, newFile, minSize, maxSize)
}

// BenchmarkShareFromEdsFile/Size:32/Axis:row/squareHalf:first(original)-10         	   17850	     66716 ns/op
// BenchmarkShareFromEdsFile/Size:32/Axis:row/squareHalf:second(extended)-10        	   18517	     64462 ns/op
// BenchmarkShareFromEdsFile/Size:32/Axis:col/squareHalf:first(original)-10         	   10000	    104241 ns/op
// BenchmarkShareFromEdsFile/Size:32/Axis:col/squareHalf:second(extended)-10        	   10000	    101964 ns/op
// BenchmarkShareFromEdsFile/Size:64/Axis:row/squareHalf:first(original)-10         	    8641	    129674 ns/op
// BenchmarkShareFromEdsFile/Size:64/Axis:row/squareHalf:second(extended)-10        	    9022	    124899 ns/op
// BenchmarkShareFromEdsFile/Size:64/Axis:col/squareHalf:first(original)-10         	    5625	    204934 ns/op
// BenchmarkShareFromEdsFile/Size:64/Axis:col/squareHalf:second(extended)-10        	    5785	    200634 ns/op
// BenchmarkShareFromEdsFile/Size:128/Axis:row/squareHalf:first(original)-10        	    4424	    262753 ns/op
// BenchmarkShareFromEdsFile/Size:128/Axis:row/squareHalf:second(extended)-10       	    4690	    252676 ns/op
// BenchmarkShareFromEdsFile/Size:128/Axis:col/squareHalf:first(original)-10        	    2834	    415072 ns/op
// BenchmarkShareFromEdsFile/Size:128/Axis:col/squareHalf:second(extended)-10       	    2934	    426160 ns/op
func BenchmarkShareFromEdsFile(b *testing.B) {
	minSize, maxSize := 32, 128
	dir := b.TempDir()
	newFile := func(size int) File {
		eds := edstest.RandEDS(b, size)
		path := dir + "/testfile"
		f, err := CreateEdsFile(path, eds)
		require.NoError(b, err)
		return f
	}
	benchGetShareFromFile(b, newFile, minSize, maxSize)
}
