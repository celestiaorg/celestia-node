package store

import (
	"testing"

	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/share/eds/edstest"
)

func TestMemFile(t *testing.T) {
	size := 32
	createMemFile := func(eds *rsmt2d.ExtendedDataSquare) File {
		return &MemFile{Eds: eds}
	}

	t.Run("Share", func(t *testing.T) {
		testFileShare(t, createMemFile, size)
	})

	t.Run("AxisHalf", func(t *testing.T) {
		testFileAxisHalf(t, createMemFile, size)
	})

	t.Run("Data", func(t *testing.T) {
		testFileDate(t, createMemFile, size)
	})

	t.Run("EDS", func(t *testing.T) {
		testFileEds(t, createMemFile, size)
	})
}

// BenchmarkAxisFromMemFile/Size:32/Axis:row/squareHalf:first(original)-10         	  269438	      4743 ns/op
// BenchmarkAxisFromMemFile/Size:32/Axis:row/squareHalf:second(extended)-10        	  258612	      4540 ns/op
// BenchmarkAxisFromMemFile/Size:32/Axis:col/squareHalf:first(original)-10         	  245673	      4312 ns/op
// BenchmarkAxisFromMemFile/Size:32/Axis:col/squareHalf:second(extended)-10        	  274141	      4541 ns/op
// BenchmarkAxisFromMemFile/Size:64/Axis:row/squareHalf:first(original)-10         	  132518	      9809 ns/op
// BenchmarkAxisFromMemFile/Size:64/Axis:row/squareHalf:second(extended)-10        	  132085	      9833 ns/op
// BenchmarkAxisFromMemFile/Size:64/Axis:col/squareHalf:first(original)-10         	  112770	      9613 ns/op
// BenchmarkAxisFromMemFile/Size:64/Axis:col/squareHalf:second(extended)-10        	  114934	      9927 ns/op
// BenchmarkAxisFromMemFile/Size:128/Axis:row/squareHalf:first(original)-10        	   68439	     19694 ns/op
// BenchmarkAxisFromMemFile/Size:128/Axis:row/squareHalf:second(extended)-10       	   64341	     20275 ns/op
// BenchmarkAxisFromMemFile/Size:128/Axis:col/squareHalf:first(original)-10        	   66495	     20180 ns/op
// BenchmarkAxisFromMemFile/Size:128/Axis:col/squareHalf:second(extended)-10       	   61392	     20912 ns/op
func BenchmarkAxisFromMemFile(b *testing.B) {
	minSize, maxSize := 32, 128
	newFile := func(size int) File {
		eds := edstest.RandEDS(b, size)
		return &MemFile{Eds: eds}
	}
	benchGetAxisFromFile(b, newFile, minSize, maxSize)
}

// BenchmarkShareFromMemFile/Size:32/Axis:row/squareHalf:first(original)-10         	   17586	     66750 ns/op
// BenchmarkShareFromMemFile/Size:32/Axis:row/squareHalf:second(extended)-10        	   18468	     68188 ns/op
// BenchmarkShareFromMemFile/Size:32/Axis:col/squareHalf:first(original)-10         	   17899	     66697 ns/op
// BenchmarkShareFromMemFile/Size:32/Axis:col/squareHalf:second(extended)-10        	   18092	     65383 ns/op
// BenchmarkShareFromMemFile/Size:64/Axis:row/squareHalf:first(original)-10         	    8922	    135033 ns/op
// BenchmarkShareFromMemFile/Size:64/Axis:row/squareHalf:second(extended)-10        	    9652	    130358 ns/op
// BenchmarkShareFromMemFile/Size:64/Axis:col/squareHalf:first(original)-10         	    8041	    130971 ns/op
// BenchmarkShareFromMemFile/Size:64/Axis:col/squareHalf:second(extended)-10        	    8778	    127361 ns/op
// BenchmarkShareFromMemFile/Size:128/Axis:row/squareHalf:first(original)-10        	    4464	    260158 ns/op
// BenchmarkShareFromMemFile/Size:128/Axis:row/squareHalf:second(extended)-10       	    4464	    248248 ns/op
// BenchmarkShareFromMemFile/Size:128/Axis:col/squareHalf:first(original)-10        	    4486	    257392 ns/op
// BenchmarkShareFromMemFile/Size:128/Axis:col/squareHalf:second(extended)-10       	    4335	    248022 ns/op
func BenchmarkShareFromMemFile(b *testing.B) {
	minSize, maxSize := 32, 128
	newFile := func(size int) File {
		eds := edstest.RandEDS(b, size)
		return &MemFile{Eds: eds}
	}

	benchGetShareFromFile(b, newFile, minSize, maxSize)
}
