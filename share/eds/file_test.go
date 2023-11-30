package eds

import (
	"context"
	"crypto/sha256"
	"fmt"
	"runtime"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds/edstest"
)

func TestCreateFile(t *testing.T) {
	path := t.TempDir() + "/testfile"
	edsIn := edstest.RandEDS(t, 8)

	for _, mode := range []FileMode{EDSMode, ODSMode} {
		f, err := CreateFile(path, edsIn, FileConfig{Mode: mode})
		require.NoError(t, err)
		edsOut, err := f.EDS()
		require.NoError(t, err)
		assert.True(t, edsIn.Equals(edsOut))
	}
}

func TestFile(t *testing.T) {
	path := t.TempDir() + "/testfile"
	eds := edstest.RandEDS(t, 8)
	root, err := share.NewRoot(eds)
	require.NoError(t, err)

	// TODO(@Wondartan): Test in multiple modes
	fl, err := CreateFile(path, eds)
	require.NoError(t, err)
	err = fl.Close()
	require.NoError(t, err)

	fl, err = OpenFile(path)
	require.NoError(t, err)

	axis := []rsmt2d.Axis{rsmt2d.Col, rsmt2d.Row}
	for _, axis := range axis {
		for i := 0; i < int(eds.Width()); i++ {
			row, err := fl.Axis(i, axis)
			require.NoError(t, err)
			assert.EqualValues(t, getAxis(i, axis, eds), row)
		}
	}

	width := int(eds.Width())
	for _, axis := range axis {
		for i := 0; i < width*width; i++ {
			row, col := uint(i/width), uint(i%width)
			shr, err := fl.ShareWithProof(context.TODO(), i, axis, nil)
			require.NoError(t, err)
			assert.EqualValues(t, eds.GetCell(row, col), shr)

			namespace := share.ParitySharesNamespace
			if int(row) < width/2 && int(col) < width/2 {
				namespace = share.GetNamespace(shr.Share)
			}

			axishash := root.RowRoots[row]
			if axis == rsmt2d.Col {
				axishash = root.ColumnRoots[col]
			}

			ok := shr.Proof.VerifyInclusion(sha256.New(), namespace.ToNMT(), [][]byte{shr.Share}, axishash)
			assert.True(t, ok)
		}
	}

	out, err := fl.EDS()
	require.NoError(t, err)
	assert.True(t, eds.Equals(out))

	err = fl.Close()
	require.NoError(t, err)
}

// BenchmarkGetShareFromDisk/16 	   32978	     35484 ns/op
// BenchmarkGetShareFromDisk/32 	   17452	     68491 ns/op
// BenchmarkGetShareFromDisk/64 	    8184	    131113 ns/op
// BenchmarkGetShareFromDisk/128	    4574	    265150 ns/op
func BenchmarkGetShareFromDisk(b *testing.B) {
	minSize, maxSize := 16, 128
	dir := b.TempDir()
	newFile := func(size int) File {
		sqr := edstest.RandEDS(b, size)
		f, err := CreateFile(dir+"/"+strconv.Itoa(size), sqr)
		require.NoError(b, err)
		return f
	}

	benchGetShareFromFile(b, newFile, minSize, maxSize)
}

// BenchmarkGetShareFromMem/16 	   30717	     35675 ns/op
// BenchmarkGetShareFromMem/32 	   17617	     69203 ns/op
// BenchmarkGetShareFromMem/64 	    7988	    134039 ns/op
// BenchmarkGetShareFromMem/128	    4582	    264621 ns/op
func BenchmarkGetShareFromMem(b *testing.B) {
	minSize, maxSize := 16, 128
	newFile := func(size int) File {
		sqr := edstest.RandEDS(b, size)
		return &MemFile{Eds: sqr}
	}

	benchGetShareFromFile(b, newFile, minSize, maxSize)
}

// BenchmarkGetShareFromCache/16 	  655171	      1723 ns/op
// BenchmarkGetShareFromCache/32 	  568992	      1998 ns/op
// BenchmarkGetShareFromCache/64 	  530446	      2252 ns/op
// BenchmarkGetShareFromCache/128 	  519739	      2515 ns/op
func BenchmarkGetShareFromCache(b *testing.B) {
	minSize, maxSize := 16, 128
	newFile := func(size int) File {
		sqr := edstest.RandEDS(b, size)
		return NewCacheFile(&MemFile{Eds: sqr}, rsmt2d.NewLeoRSCodec())
	}

	benchGetShareFromFile(b, newFile, minSize, maxSize)
}

func TestCachedAxis(t *testing.T) {
	sqr := edstest.RandEDS(t, 32)
	mem := &MemFile{Eds: sqr}
	file := NewCacheFile(mem, rsmt2d.NewLeoRSCodec())

	for i := 0; i < mem.Size(); i++ {
		a1, err := mem.Axis(i, rsmt2d.Row)
		require.NoError(t, err)
		a2, err := file.Axis(i, rsmt2d.Row)
		require.NoError(t, err)
		require.Equal(t, a1, a2)
	}
}

func TestCachedEDS(t *testing.T) {
	sqr := edstest.RandEDS(t, 32)
	mem := &MemFile{Eds: sqr}
	file := NewCacheFile(mem, rsmt2d.NewLeoRSCodec())

	e1, err := mem.EDS()
	require.NoError(t, err)
	e2, err := file.EDS()
	require.NoError(t, err)

	r1, err := e1.RowRoots()
	require.NoError(t, err)
	r2, err := e2.RowRoots()
	require.NoError(t, err)

	require.Equal(t, r1, r2)
}

// BenchmarkGetShareFromCacheMiss/16 	   16308	     72295 ns/op
// BenchmarkGetShareFromCacheMiss/32 	    8216	    141334 ns/op
// BenchmarkGetShareFromCacheMiss/64 	    3877	    284171 ns/op
// BenchmarkGetShareFromCacheMiss/128  	    2146	    552244 ns/op
func BenchmarkGetShareFromCacheMiss(b *testing.B) {
	minSize, maxSize := 16, 128
	newFile := func(size int) File {
		sqr := edstest.RandEDS(b, size)
		f := NewCacheFile(&MemFile{Eds: sqr}, rsmt2d.NewLeoRSCodec())
		f.disableCache = true
		return f
	}

	benchGetShareFromFile(b, newFile, minSize, maxSize)
}

func TestCacheMemoryUsageMany(t *testing.T) {
	for i := 0; i < 10; i++ {
		TestCacheMemoryUsage(t)
	}
}

// Allocations in kB
// Mem size before: 370802
// Mem size after: 982305
// Diff between snapshots:
// Alloc size: 611502 TotalAlloc (without GC): 1544656 Heap size: 611502
func TestCacheMemoryUsage(t *testing.T) {
	size := 128
	amount := 10
	dir := t.TempDir()

	type test struct {
		file File
		rows [][]byte
	}
	tests := make([]test, amount)
	for i := range tests {
		eds := edstest.RandEDS(t, size)
		df, err := CreateFile(dir+"/"+strconv.Itoa(i), eds)
		require.NoError(t, err)
		f := NewCacheFile(df, rsmt2d.NewLeoRSCodec())

		rows, err := eds.RowRoots()
		require.NoError(t, err)
		tests[i] = test{
			file: f,
			rows: rows,
		}
	}

	for sample := 0; sample < 2; sample++ {
		var m1, m2 runtime.MemStats
		runtime.GC()
		runtime.ReadMemStats(&m1)

		for _, test := range tests {
			for i := 0; i < size; i++ {
				_, err := test.file.ShareWithProof(context.TODO(), 2*i*size, rsmt2d.Row, test.rows[i])
				require.NoError(t, err)
			}
		}

		runtime.GC()
		runtime.ReadMemStats(&m2)
		memUsage(&m1, &m2)
	}
}

func memUsage(m1, m2 *runtime.MemStats) {
	fmt.Println("Mem size before:", (m1.Alloc)/1024)

	fmt.Println("Mem size after:", (m2.Alloc)/1024)

	fmt.Println("Diff between snapshots:\n",
		"Alloc size:", (int(m2.Alloc)-int(m1.Alloc))/1024,
		"TotalAlloc (without GC):", (int(m2.TotalAlloc)-int(m1.TotalAlloc))/1024,
		"Heap size:", (int(m2.HeapAlloc)-int(m1.HeapAlloc))/1024,
	)
}

func benchGetShareFromFile(b *testing.B, newFile func(size int) File, minSize, maxSize int) {
	for size := minSize; size <= maxSize; size *= 2 {
		name := strconv.Itoa(size)
		b.Run(name, func(b *testing.B) {
			f := newFile(size)

			sqr, err := f.EDS()
			require.NoError(b, err)
			roots, err := sqr.RowRoots()
			require.NoError(b, err)

			// warm up cache
			_, err = f.ShareWithProof(context.TODO(), 0, rsmt2d.Row, roots[0])
			require.NoError(b, err)

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, err = f.ShareWithProof(context.TODO(), 0, rsmt2d.Row, roots[0])
				require.NoError(b, err)
			}
		})
	}
}
