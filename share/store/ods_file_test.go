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
	codec := rsmt2d.NewLeoRSCodec()
	mem := newMemPool(codec, int(edsIn.Width()))
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
	codec := rsmt2d.NewLeoRSCodec()
	mem := newMemPool(codec, size)
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

// BenchmarkAxisFromOdsFile/Size:32/Axis:row/squareHalf:first(original)-10         	  429498	      2464 ns/op
// BenchmarkAxisFromOdsFile/Size:32/Axis:row/squareHalf:second(extended)-10        	    5889	    192904 ns/op
// BenchmarkAxisFromOdsFile/Size:32/Axis:col/squareHalf:first(original)-10         	   56209	     20926 ns/op
// BenchmarkAxisFromOdsFile/Size:32/Axis:col/squareHalf:second(extended)-10        	    5480	    193249 ns/op
// BenchmarkAxisFromOdsFile/Size:64/Axis:row/squareHalf:first(original)-10         	  287070	      4003 ns/op
// BenchmarkAxisFromOdsFile/Size:64/Axis:row/squareHalf:second(extended)-10        	    2212	    506601 ns/op
// BenchmarkAxisFromOdsFile/Size:64/Axis:col/squareHalf:first(original)-10         	   28990	     41353 ns/op
// BenchmarkAxisFromOdsFile/Size:64/Axis:col/squareHalf:second(extended)-10        	    2358	    511020 ns/op
// BenchmarkAxisFromOdsFile/Size:128/Axis:row/squareHalf:first(original)-10        	  186265	      6309 ns/op
// BenchmarkAxisFromOdsFile/Size:128/Axis:row/squareHalf:second(extended)-10       	     610	   1814819 ns/op
// BenchmarkAxisFromOdsFile/Size:128/Axis:col/squareHalf:first(original)-10        	   14460	     82613 ns/op
// BenchmarkAxisFromOdsFile/Size:128/Axis:col/squareHalf:second(extended)-10       	     640	   1819996 ns/op
func BenchmarkAxisFromOdsFile(b *testing.B) {
	minSize, maxSize := 32, 128
	dir := b.TempDir()
	codec := rsmt2d.NewLeoRSCodec()
	mem := make(map[int]memPool)
	for i := minSize; i <= maxSize; i *= 2 {
		mem[i] = newMemPool(codec, i)
	}

	newFile := func(size int) File {
		eds := edstest.RandEDS(b, size)
		path := dir + "/testfile"
		f, err := CreateOdsFile(path, eds, mem[size])
		require.NoError(b, err)
		return f
	}
	benchGetAxisFromFile(b, newFile, minSize, maxSize)
}

// BenchmarkShareFromOdsFile/Size:32/Axis:row/squareHalf:first(original)-10         	   10333	    113351 ns/op
// BenchmarkShareFromOdsFile/Size:32/Axis:row/squareHalf:second(extended)-10        	    3794	    319437 ns/op
// BenchmarkShareFromOdsFile/Size:32/Axis:col/squareHalf:first(original)-10         	    7201	    139066 ns/op
// BenchmarkShareFromOdsFile/Size:32/Axis:col/squareHalf:second(extended)-10        	    3612	    317520 ns/op
// BenchmarkShareFromOdsFile/Size:64/Axis:row/squareHalf:first(original)-10         	    5462	    220543 ns/op
// BenchmarkShareFromOdsFile/Size:64/Axis:row/squareHalf:second(extended)-10        	    1586	    775291 ns/op
// BenchmarkShareFromOdsFile/Size:64/Axis:col/squareHalf:first(original)-10         	    4611	    257328 ns/op
// BenchmarkShareFromOdsFile/Size:64/Axis:col/squareHalf:second(extended)-10        	    1534	    788619 ns/op
// BenchmarkShareFromOdsFile/Size:128/Axis:row/squareHalf:first(original)-10        	    2413	    448675 ns/op
// BenchmarkShareFromOdsFile/Size:128/Axis:row/squareHalf:second(extended)-10       	     517	   2427473 ns/op
// BenchmarkShareFromOdsFile/Size:128/Axis:col/squareHalf:first(original)-10        	    2200	    528681 ns/op
// BenchmarkShareFromOdsFile/Size:128/Axis:col/squareHalf:second(extended)-10       	     464	   2385446 ns/op
func BenchmarkShareFromOdsFile(b *testing.B) {
	minSize, maxSize := 32, 128
	dir := b.TempDir()
	codec := rsmt2d.NewLeoRSCodec()
	mem := make(map[int]memPool)
	for i := minSize; i <= maxSize; i *= 2 {
		mem[i] = newMemPool(codec, i)
	}

	newFile := func(size int) File {
		eds := edstest.RandEDS(b, size)
		path := dir + "/testfile"
		f, err := CreateOdsFile(path, eds, mem[size])
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
