package file

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/libs/rand"

	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds/edstest"
	eds "github.com/celestiaorg/celestia-node/share/new_eds"
)

func TestCreateODSFile(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)

	edsIn := edstest.RandEDS(t, 8)
	f := createODSFile(t, edsIn)
	readRoots, err := share.NewAxisRoots(edsIn)
	require.NoError(t, err)

	shares, err := f.Shares(ctx)
	require.NoError(t, err)

	expected := edsIn.FlattenedODS()
	require.Equal(t, expected, shares)

	roots, err := f.AxisRoots(ctx)
	require.NoError(t, err)
	require.Equal(t, share.DataHash(roots.Hash()), f.hdr.datahash)
	require.True(t, roots.Equals(readRoots))
	require.NoError(t, f.Close())
}

func TestReadODSFromFile(t *testing.T) {
	eds := edstest.RandEDS(t, 8)
	f := createODSFile(t, eds)

	ods, err := f.readODS()
	require.NoError(t, err)
	for i, row := range ods {
		original := eds.Row(uint(i))[:eds.Width()/2]
		require.True(t, len(original) == len(row))
		require.Equal(t, original, row)
	}
}

func TestODSFile(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	t.Cleanup(cancel)

	ODSSize := 16
	eds.TestSuiteAccessor(ctx, t, createAccessor, ODSSize)
	eds.TestStreamer(ctx, t, createCachedStreamer, ODSSize)
	eds.TestStreamer(ctx, t, createStreamer, ODSSize)
}

// BenchmarkAxisFromODSFile/Size:32/ProofType:row/squareHalf:0-16         	  382011	      3104 ns/op
// BenchmarkAxisFromODSFile/Size:32/ProofType:row/squareHalf:1-16         	    9320	    122408 ns/op
// BenchmarkAxisFromODSFile/Size:32/ProofType:col/squareHalf:0-16         	 4408911	       266.5 ns/op
// BenchmarkAxisFromODSFile/Size:32/ProofType:col/squareHalf:1-16         	    9488	    119472 ns/op
// BenchmarkAxisFromODSFile/Size:64/ProofType:row/squareHalf:0-16         	  240913	      5239 ns/op
// BenchmarkAxisFromODSFile/Size:64/ProofType:row/squareHalf:1-16         	    1018	   1249622 ns/op
// BenchmarkAxisFromODSFile/Size:64/ProofType:col/squareHalf:0-16         	 2614063	       451.8 ns/op
// BenchmarkAxisFromODSFile/Size:64/ProofType:col/squareHalf:1-16         	    1917	    661510 ns/op
// BenchmarkAxisFromODSFile/Size:128/ProofType:row/squareHalf:0-16        	  119324	     10425 ns/op
// BenchmarkAxisFromODSFile/Size:128/ProofType:row/squareHalf:1-16        	     163	   9926752 ns/op
// BenchmarkAxisFromODSFile/Size:128/ProofType:col/squareHalf:0-16        	 1634124	       726.2 ns/op
// BenchmarkAxisFromODSFile/Size:128/ProofType:col/squareHalf:1-16        	     205	   5508394 ns/op
func BenchmarkAxisFromODSFile(b *testing.B) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	b.Cleanup(cancel)

	minSize, maxSize := 32, 128
	newFile := func(size int) eds.Accessor {
		eds := edstest.RandEDS(b, size)
		return createODSFile(b, eds)
	}
	eds.BenchGetHalfAxisFromAccessor(ctx, b, newFile, minSize, maxSize)
}

// BenchmarkAxisFromODSFileDisabledCache/Size:32/ProofType:row/squareHalf:0-16         	  378975	      3141 ns/op
// BenchmarkAxisFromODSFileDisabledCache/Size:32/ProofType:row/squareHalf:1-16         	    1026	   1175651 ns/op
// BenchmarkAxisFromODSFileDisabledCache/Size:32/ProofType:col/squareHalf:0-16         	   80200	     14721 ns/op
// BenchmarkAxisFromODSFileDisabledCache/Size:32/ProofType:col/squareHalf:1-16         	    1014	   1180527 ns/op
// BenchmarkAxisFromODSFileDisabledCache/Size:64/ProofType:row/squareHalf:0-16         	  212041	      5417 ns/op
// BenchmarkAxisFromODSFileDisabledCache/Size:64/ProofType:row/squareHalf:1-16         	     253	   4205953 ns/op
// BenchmarkAxisFromODSFileDisabledCache/Size:64/ProofType:col/squareHalf:0-16         	   35289	     34033 ns/op
// BenchmarkAxisFromODSFileDisabledCache/Size:64/ProofType:col/squareHalf:1-16         	     325	   3229517 ns/op
// BenchmarkAxisFromODSFileDisabledCache/Size:128/ProofType:row/squareHalf:0-16        	  132261	      8535 ns/op
// BenchmarkAxisFromODSFileDisabledCache/Size:128/ProofType:row/squareHalf:1-16        	      48	  22963229 ns/op
// BenchmarkAxisFromODSFileDisabledCache/Size:128/ProofType:col/squareHalf:0-16        	   19053	     62858 ns/op
// BenchmarkAxisFromODSFileDisabledCache/Size:128/ProofType:col/squareHalf:1-16        	      48	  21185201 ns/op
func BenchmarkAxisFromODSFileDisabledCache(b *testing.B) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	b.Cleanup(cancel)

	minSize, maxSize := 32, 128
	newFile := func(size int) eds.Accessor {
		eds := edstest.RandEDS(b, size)
		return createODSFileDisabledCache(b, eds)
	}
	eds.BenchGetHalfAxisFromAccessor(ctx, b, newFile, minSize, maxSize)
}

// BenchmarkSampleFromODSFile/Size:32/quadrant:1-16         	   13684	     87558 ns/op
// BenchmarkSampleFromODSFile/Size:32/quadrant:2-16         	   13358	     85677 ns/op
// BenchmarkSampleFromODSFile/Size:32/quadrant:3-16         	   10000	    102631 ns/op
// BenchmarkSampleFromODSFile/Size:32/quadrant:4-16         	    5175	    222615 ns/op
// BenchmarkSampleFromODSFile/Size:64/quadrant:1-16         	    7142	    173784 ns/op
// BenchmarkSampleFromODSFile/Size:64/quadrant:2-16         	    6820	    171602 ns/op
// BenchmarkSampleFromODSFile/Size:64/quadrant:3-16         	    5232	    201875 ns/op
// BenchmarkSampleFromODSFile/Size:64/quadrant:4-16         	    1448	   1035275 ns/op
// BenchmarkSampleFromODSFile/Size:128/quadrant:1-16        	    3829	    359528 ns/op
// BenchmarkSampleFromODSFile/Size:128/quadrant:2-16        	    3303	    358142 ns/op
// BenchmarkSampleFromODSFile/Size:128/quadrant:3-16        	    2666	    431895 ns/op
// BenchmarkSampleFromODSFile/Size:128/quadrant:4-16        	     183	   7347936 ns/op
func BenchmarkSampleFromODSFile(b *testing.B) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	b.Cleanup(cancel)

	minSize, maxSize := 32, 128
	newFile := func(size int) eds.Accessor {
		eds := edstest.RandEDS(b, size)
		return createODSFile(b, eds)
	}
	eds.BenchGetSampleFromAccessor(ctx, b, newFile, minSize, maxSize)
}

// BenchmarkSampleFromODSFileDisabledCache/Size:32/quadrant:1
// BenchmarkSampleFromODSFileDisabledCache/Size:32/quadrant:1-16         	   13152	     85301 ns/op
// BenchmarkSampleFromODSFileDisabledCache/Size:32/quadrant:2-16         	   14140	     84876 ns/op
// BenchmarkSampleFromODSFileDisabledCache/Size:32/quadrant:3-16         	   11756	    102360 ns/op
// BenchmarkSampleFromODSFileDisabledCache/Size:32/quadrant:4-16         	     985	   1292232 ns/op
// BenchmarkSampleFromODSFileDisabledCache/Size:64/quadrant:1-16         	    7678	    172306 ns/op
// BenchmarkSampleFromODSFileDisabledCache/Size:64/quadrant:2-16         	    5744	    176533 ns/op
// BenchmarkSampleFromODSFileDisabledCache/Size:64/quadrant:3-16         	    6022	    207884 ns/op
// BenchmarkSampleFromODSFileDisabledCache/Size:64/quadrant:4-16         	     304	   3881858 ns/op
// BenchmarkSampleFromODSFileDisabledCache/Size:128/quadrant:1-16        	    3697	    355835 ns/op
// BenchmarkSampleFromODSFileDisabledCache/Size:128/quadrant:2-16        	    3558	    360162 ns/op
// BenchmarkSampleFromODSFileDisabledCache/Size:128/quadrant:3-16        	    3027	    410976 ns/op
// BenchmarkSampleFromODSFileDisabledCache/Size:128/quadrant:4-16        	      54	  21796460 ns/op
func BenchmarkSampleFromODSFileDisabledCache(b *testing.B) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	b.Cleanup(cancel)

	minSize, maxSize := 32, 128
	newFile := func(size int) eds.Accessor {
		eds := edstest.RandEDS(b, size)
		return createODSFileDisabledCache(b, eds)
	}
	eds.BenchGetSampleFromAccessor(ctx, b, newFile, minSize, maxSize)
}

func createAccessor(t testing.TB, eds *rsmt2d.ExtendedDataSquare) eds.Accessor {
	return createODSFile(t, eds)
}

func createStreamer(t testing.TB, eds *rsmt2d.ExtendedDataSquare) eds.AccessorStreamer {
	return createODSFile(t, eds)
}

func createCachedStreamer(t testing.TB, eds *rsmt2d.ExtendedDataSquare) eds.AccessorStreamer {
	f := createODSFile(t, eds)
	_, err := f.readODS()
	require.NoError(t, err)
	return f
}

func createODSFile(t testing.TB, eds *rsmt2d.ExtendedDataSquare) *ODS {
	path := t.TempDir() + "/" + strconv.Itoa(rand.Intn(1000))
	roots, err := share.NewAxisRoots(eds)
	require.NoError(t, err)
	err = CreateODS(path, roots, eds)
	require.NoError(t, err)
	ods, err := OpenODS(path)
	return ods
}

func createODSFileDisabledCache(t testing.TB, eds *rsmt2d.ExtendedDataSquare) eds.Accessor {
	path := t.TempDir() + "/" + strconv.Itoa(rand.Intn(1000))
	roots, err := share.NewAxisRoots(eds)
	require.NoError(t, err)
	err = CreateODS(path, roots, eds)
	require.NoError(t, err)
	ods, err := OpenODS(path)
	ods.disableCache = true
	return ods
}
