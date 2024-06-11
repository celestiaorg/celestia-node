package file

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/libs/rand"

	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds/edstest"
	eds "github.com/celestiaorg/celestia-node/share/new_eds"
)

func TestCreateOdsFile(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)

	path := t.TempDir() + "/testfile"
	edsIn := edstest.RandEDS(t, 8)
	datahash := share.DataHash(rand.Bytes(32))
	f, err := CreateOdsFile(path, datahash, edsIn)
	require.NoError(t, err)

	shares, err := f.Shares(ctx)
	require.NoError(t, err)
	expected := edsIn.FlattenedODS()
	require.Equal(t, expected, shares)
	require.Equal(t, datahash, f.hdr.datahash)
	require.NoError(t, f.Close())

	f, err = OpenOdsFile(path)
	require.NoError(t, err)
	shares, err = f.Shares(ctx)
	require.NoError(t, err)
	require.Equal(t, expected, shares)
	require.Equal(t, datahash, f.hdr.datahash)
	require.NoError(t, f.Close())
}

func TestReadOdsFromFile(t *testing.T) {
	eds := edstest.RandEDS(t, 8)
	path := t.TempDir() + "/testfile"
	f, err := CreateOdsFile(path, []byte{}, eds)
	require.NoError(t, err)

	err = f.readOds()
	require.NoError(t, err)
	for i, row := range f.ods {
		original := eds.Row(uint(i))[:eds.Width()/2]
		require.True(t, len(original) == len(row))
		require.Equal(t, original, row)
	}
}

func TestOdsFile(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)

	odsSize := 8
	createOdsFile := func(eds *rsmt2d.ExtendedDataSquare) eds.Accessor {
		path := t.TempDir() + "/testfile"
		fl, err := CreateOdsFile(path, []byte{}, eds)
		require.NoError(t, err)
		return fl
	}

	eds.TestSuiteAccessor(ctx, t, createOdsFile, odsSize)
}

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
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	b.Cleanup(cancel)

	minSize, maxSize := 32, 128
	dir := b.TempDir()

	newFile := func(size int) eds.Accessor {
		eds := edstest.RandEDS(b, size)
		path := dir + "/testfile"
		f, err := CreateOdsFile(path, []byte{}, eds)
		require.NoError(b, err)
		return f
	}
	eds.BenchGetHalfAxisFromAccessor(ctx, b, newFile, minSize, maxSize)
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
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	b.Cleanup(cancel)

	minSize, maxSize := 32, 128
	dir := b.TempDir()

	newFile := func(size int) eds.Accessor {
		eds := edstest.RandEDS(b, size)
		path := dir + "/testfile"
		f, err := CreateOdsFile(path, []byte{}, eds)
		require.NoError(b, err)
		return f
	}

	eds.BenchGetSampleFromAccessor(ctx, b, newFile, minSize, maxSize)
}
