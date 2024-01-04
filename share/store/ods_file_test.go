package store

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/share/eds/edstest"
)

func TestCreateOdsFile(t *testing.T) {
	path := t.TempDir() + "/testfile"
	edsIn := edstest.RandEDS(t, 8)
	mem := newMemPools(NewCodec())
	_, err := CreateOdsFile(path, edsIn, mem)
	require.NoError(t, err)

	f, err := OpenOdsFile(path)
	require.NoError(t, err)
	edsOut, err := f.EDS(context.TODO())
	require.NoError(t, err)
	assert.True(t, edsIn.Equals(edsOut))
}

func TestOdsFile(t *testing.T) {
	size := 32
	mem := newMemPools(NewCodec())
	createOdsFile := func(eds *rsmt2d.ExtendedDataSquare) File {
		path := t.TempDir() + "/testfile"
		fl, err := CreateOdsFile(path, eds, mem)
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

func TestReadOdsFile(t *testing.T) {
	eds := edstest.RandEDS(t, 8)
	mem := newMemPools(NewCodec())
	path := t.TempDir() + "/testfile"
	f, err := CreateOdsFile(path, eds, mem)
	require.NoError(t, err)

	ods, err := f.readOds(rsmt2d.Row)
	require.NoError(t, err)
	for i, row := range ods.square {
		original, err := f.readRow(i)
		require.NoError(t, err)
		require.True(t, len(original) == len(row))
		require.Equal(t, original, row)
	}
}

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
func BenchmarkAxisFromOdsFile(b *testing.B) {
	minSize, maxSize := 32, 128
	dir := b.TempDir()
	mem := newMemPools(NewCodec())

	newFile := func(size int) File {
		eds := edstest.RandEDS(b, size)
		path := dir + "/testfile"
		f, err := CreateOdsFile(path, eds, mem)
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
	minSize, maxSize := 32, 128
	dir := b.TempDir()
	mem := newMemPools(NewCodec())

	newFile := func(size int) File {
		eds := edstest.RandEDS(b, size)
		path := dir + "/testfile"
		f, err := CreateOdsFile(path, eds, mem)
		require.NoError(b, err)
		return f
	}

	benchGetShareFromFile(b, newFile, minSize, maxSize)
}

type squareHalf int

func (q squareHalf) String() string {
	switch q {
	case 0:
		return "first(original)"
	case 1:
		return "second(extended)"
	}
	return "unknown"
}

func benchGetAxisFromFile(b *testing.B, newFile func(size int) File, minSize, maxSize int) {
	for size := minSize; size <= maxSize; size *= 2 {
		f := newFile(size)

		// loop over all possible axis types and quadrants
		for _, axisType := range []rsmt2d.Axis{rsmt2d.Row, rsmt2d.Col} {
			for _, squareHalf := range []squareHalf{0, 1} {
				name := fmt.Sprintf("Size:%v/Axis:%s/squareHalf:%s", size, axisType, squareHalf)
				b.Run(name, func(b *testing.B) {
					b.ResetTimer()
					for i := 0; i < b.N; i++ {
						_, err := f.AxisHalf(context.TODO(), axisType, f.Size()/2*int(squareHalf))
						require.NoError(b, err)
					}
				})
			}
		}
	}
}

func benchGetShareFromFile(b *testing.B, newFile func(size int) File, minSize, maxSize int) {
	for size := minSize; size <= maxSize; size *= 2 {
		f := newFile(size)

		// loop over all possible axis types and quadrants
		for _, axisType := range []rsmt2d.Axis{rsmt2d.Row, rsmt2d.Col} {
			for _, squareHalf := range []squareHalf{0, 1} {
				name := fmt.Sprintf("Size:%v/Axis:%s/squareHalf:%s", size, axisType, squareHalf)
				b.Run(name, func(b *testing.B) {
					idx := f.Size() - 1
					// warm up cache
					_, _, err := f.Share(context.TODO(), axisType, f.Size()/2*int(squareHalf), idx)
					require.NoError(b, err)

					b.ResetTimer()
					for i := 0; i < b.N; i++ {
						_, _, err = f.Share(context.TODO(), axisType, f.Size()/2*int(squareHalf), idx)
						require.NoError(b, err)
					}
				})
			}
		}
	}
}
