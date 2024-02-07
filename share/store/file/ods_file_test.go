package file

import (
	"context"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"

	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/share/eds/edstest"
)

func TestCreateOdsFile(t *testing.T) {
	path := t.TempDir() + "/testfile"
	edsIn := edstest.RandEDS(t, 8)
	_, err := CreateOdsFile(path, 1, []byte{}, edsIn)
	require.NoError(t, err)

	f, err := OpenOdsFile(path)
	require.NoError(t, err)
	edsOut, err := f.EDS(context.TODO())
	require.NoError(t, err)
	assert.True(t, edsIn.Equals(edsOut))
}

func TestOdsFile(t *testing.T) {
	size := 32
	createOdsFile := func(eds *rsmt2d.ExtendedDataSquare) EdsFile {
		path := t.TempDir() + "/testfile"
		fl, err := CreateOdsFile(path, 1, []byte{}, eds)
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

func TestReadOdsFile(t *testing.T) {
	eds := edstest.RandEDS(t, 8)
	path := t.TempDir() + "/testfile"
	f, err := CreateOdsFile(path, 1, []byte{}, eds)
	require.NoError(t, err)

	err = f.readOds()
	require.NoError(t, err)
	for i, row := range f.ods {
		original, err := f.readRow(i)
		require.NoError(t, err)
		require.True(t, len(original) == len(row))
		require.Equal(t, original, row)
	}
}

// Leopard full encode
// BenchmarkAxisFromOdsFile/Size:32/Axis:row/squareHalf:first(original)-10         	  418206	      2545 ns/op
// BenchmarkAxisFromOdsFile/Size:32/Axis:row/squareHalf:second(extended)-10        	    4968	    227265 ns/op
// BenchmarkAxisFromOdsFile/Size:32/Axis:col/squareHalf:first(original)-10         	   57007	     20707 ns/op
// BenchmarkAxisFromOdsFile/Size:32/Axis:col/squareHalf:second(extended)-10        	    5016	    214184 ns/op
// BenchmarkAxisFromOdsFile/Size:64/Axis:row/squareHalf:first(original)-10         	  308559	      3786 ns/op
// BenchmarkAxisFromOdsFile/Size:64/Axis:row/squareHalf:second(extended)-10        	    1624	    713999 ns/op
// BenchmarkAxisFromOdsFile/Size:64/Axis:col/squareHalf:first(original)-10         	   28724	     41421 ns/op
// BenchmarkAxisFromOdsFile/Size:64/Axis:col/squareHalf:second(extended)-10        	    1686	    629314 ns/op
// BenchmarkAxisFromOdsFile/Size:128/Axis:row/squareHalf:first(original)-10        	  183322	      6360 ns/op
// BenchmarkAxisFromOdsFile/Size:128/Axis:row/squareHalf:second(extended)-10       	     428	   2616150 ns/op
// BenchmarkAxisFromOdsFile/Size:128/Axis:col/squareHalf:first(original)-10        	   14338	     83598 ns/op
// BenchmarkAxisFromOdsFile/Size:128/Axis:col/squareHalf:second(extended)-10       	     488	   2213146 ns/op

// ReconstructSome, default codec
// BenchmarkAxisFromOdsFile/Size:32/Axis:row/squareHalf:first(original)-10         	  455848	      2588 ns/op
// BenchmarkAxisFromOdsFile/Size:32/Axis:row/squareHalf:second(extended)-10        	    9015	    203950 ns/op
// BenchmarkAxisFromOdsFile/Size:32/Axis:col/squareHalf:first(original)-10         	   52734	     21178 ns/op
// BenchmarkAxisFromOdsFile/Size:32/Axis:col/squareHalf:second(extended)-10        	    8830	    127452 ns/op
// BenchmarkAxisFromOdsFile/Size:64/Axis:row/squareHalf:first(original)-10         	  303834	      4763 ns/op
// BenchmarkAxisFromOdsFile/Size:64/Axis:row/squareHalf:second(extended)-10        	    2940	    426246 ns/op
// BenchmarkAxisFromOdsFile/Size:64/Axis:col/squareHalf:first(original)-10         	   27758	     42842 ns/op
// BenchmarkAxisFromOdsFile/Size:64/Axis:col/squareHalf:second(extended)-10        	    3385	    353868 ns/op
// BenchmarkAxisFromOdsFile/Size:128/Axis:row/squareHalf:first(original)-10        	  172086	      6455 ns/op
// BenchmarkAxisFromOdsFile/Size:128/Axis:row/squareHalf:second(extended)-10       	     672	   1550386 ns/op
// BenchmarkAxisFromOdsFile/Size:128/Axis:col/squareHalf:first(original)-10        	   14202	     84316 ns/op
// BenchmarkAxisFromOdsFile/Size:128/Axis:col/squareHalf:second(extended)-10       	     978	   1230980 ns/op
func BenchmarkAxisFromOdsFile(b *testing.B) {
	minSize, maxSize := 32, 128
	dir := b.TempDir()

	newFile := func(size int) EdsFile {
		eds := edstest.RandEDS(b, size)
		path := dir + "/testfile"
		f, err := CreateOdsFile(path, 1, []byte{}, eds)
		require.NoError(b, err)
		return f
	}
	benchGetAxisFromFile(b, newFile, minSize, maxSize)
}

// BenchmarkShareFromOdsFile/Size:32/Axis:row/squareHalf:first(original)-10         	   10339	    111328 ns/op
// BenchmarkShareFromOdsFile/Size:32/Axis:row/squareHalf:second(extended)-10        	    3392	    359180 ns/op
// BenchmarkShareFromOdsFile/Size:32/Axis:col/squareHalf:first(original)-10         	    8925	    131352 ns/op
// BenchmarkShareFromOdsFile/Size:32/Axis:col/squareHalf:second(extended)-10        	    3447	    346218 ns/op
// BenchmarkShareFromOdsFile/Size:64/Axis:row/squareHalf:first(original)-10         	    5503	    215833 ns/op
// BenchmarkShareFromOdsFile/Size:64/Axis:row/squareHalf:second(extended)-10        	    1231	   1001053 ns/op
// BenchmarkShareFromOdsFile/Size:64/Axis:col/squareHalf:first(original)-10         	    4711	    250001 ns/op
// BenchmarkShareFromOdsFile/Size:64/Axis:col/squareHalf:second(extended)-10        	    1315	    910079 ns/op
// BenchmarkShareFromOdsFile/Size:128/Axis:row/squareHalf:first(original)-10        	    2364	    435748 ns/op
// BenchmarkShareFromOdsFile/Size:128/Axis:row/squareHalf:second(extended)-10       	     358	   3330620 ns/op
// BenchmarkShareFromOdsFile/Size:128/Axis:col/squareHalf:first(original)-10        	    2114	    514642 ns/op
// BenchmarkShareFromOdsFile/Size:128/Axis:col/squareHalf:second(extended)-10       	     373	   3068104 ns/op
func BenchmarkShareFromOdsFile(b *testing.B) {
	minSize, maxSize := 128, 128
	dir := b.TempDir()

	newFile := func(size int) EdsFile {
		eds := edstest.RandEDS(b, size)
		path := dir + "/testfile"
		f, err := CreateOdsFile(path, 1, []byte{}, eds)
		require.NoError(b, err)
		return f
	}

	benchGetShareFromFile(b, newFile, minSize, maxSize)
}
