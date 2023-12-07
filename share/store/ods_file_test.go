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

	_, err := CreateOdsFile(path, edsIn)
	require.NoError(t, err)

	f, err := OpenOdsFile(path)
	require.NoError(t, err)
	edsOut, err := f.EDS(context.TODO())
	require.NoError(t, err)
	assert.True(t, edsIn.Equals(edsOut))
}

func TestOdsFile(t *testing.T) {
	size := 32
	createOdsFile := func(eds *rsmt2d.ExtendedDataSquare) File {
		path := t.TempDir() + "/testfile"
		fl, err := CreateOdsFile(path, eds)
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

// BenchmarkAxisFromOdsFile/Size:32/Axis:row/squareHalf:first(original)-10         	  435496	      2488 ns/op
// BenchmarkAxisFromOdsFile/Size:32/Axis:row/squareHalf:second(extended)-10        	     814	   1279260 ns/op
// BenchmarkAxisFromOdsFile/Size:32/Axis:col/squareHalf:first(original)-10         	   57886	     21029 ns/op
// BenchmarkAxisFromOdsFile/Size:32/Axis:col/squareHalf:second(extended)-10        	    2365	    493366 ns/op
// BenchmarkAxisFromOdsFile/Size:64/Axis:row/squareHalf:first(original)-10         	  272930	      3932 ns/op
// BenchmarkAxisFromOdsFile/Size:64/Axis:row/squareHalf:second(extended)-10        	     235	   4881303 ns/op
// BenchmarkAxisFromOdsFile/Size:64/Axis:col/squareHalf:first(original)-10         	   28566	     41591 ns/op
// BenchmarkAxisFromOdsFile/Size:64/Axis:col/squareHalf:second(extended)-10        	     758	   1605038 ns/op
// BenchmarkAxisFromOdsFile/Size:128/Axis:row/squareHalf:first(original)-10        	  145546	      7922 ns/op
// BenchmarkAxisFromOdsFile/Size:128/Axis:row/squareHalf:second(extended)-10       	      64	  17827662 ns/op
// BenchmarkAxisFromOdsFile/Size:128/Axis:col/squareHalf:first(original)-10        	   14073	     84737 ns/op
// BenchmarkAxisFromOdsFile/Size:128/Axis:col/squareHalf:second(extended)-10       	     127	  11064373 ns/op
func BenchmarkAxisFromOdsFile(b *testing.B) {
	minSize, maxSize := 32, 128
	dir := b.TempDir()
	newFile := func(size int) File {
		eds := edstest.RandEDS(b, size)
		path := dir + "/testfile"
		f, err := CreateOdsFile(path, eds)
		require.NoError(b, err)
		return f
	}
	benchGetAxisFromFile(b, newFile, minSize, maxSize)
}

// BenchmarkShareFromOdsFile/Size:32/Axis:row/squareHalf:first(original)-10         	   10316	    111701 ns/op
// BenchmarkShareFromOdsFile/Size:32/Axis:row/squareHalf:second(extended)-10        	     778	   1352715 ns/op
// BenchmarkShareFromOdsFile/Size:32/Axis:col/squareHalf:first(original)-10         	    8174	    130810 ns/op
// BenchmarkShareFromOdsFile/Size:32/Axis:col/squareHalf:second(extended)-10        	    1890	    646434 ns/op
// BenchmarkShareFromOdsFile/Size:64/Axis:row/squareHalf:first(original)-10         	    4935	    214392 ns/op
// BenchmarkShareFromOdsFile/Size:64/Axis:row/squareHalf:second(extended)-10        	     235	   5023812 ns/op
// BenchmarkShareFromOdsFile/Size:64/Axis:col/squareHalf:first(original)-10         	    4323	    252924 ns/op
// BenchmarkShareFromOdsFile/Size:64/Axis:col/squareHalf:second(extended)-10        	     567	   1870541 ns/op
// BenchmarkShareFromOdsFile/Size:128/Axis:row/squareHalf:first(original)-10        	    2424	    452331 ns/op
// BenchmarkShareFromOdsFile/Size:128/Axis:row/squareHalf:second(extended)-10       	      66	  21867956 ns/op
// BenchmarkShareFromOdsFile/Size:128/Axis:col/squareHalf:first(original)-10        	    2100	    542252 ns/op
// BenchmarkShareFromOdsFile/Size:128/Axis:col/squareHalf:second(extended)-10       	     100	  14112671 ns/op
func BenchmarkShareFromOdsFile(b *testing.B) {
	minSize, maxSize := 32, 128
	dir := b.TempDir()
	newFile := func(size int) File {
		eds := edstest.RandEDS(b, size)
		path := dir + "/testfile"
		f, err := CreateOdsFile(path, eds)
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
