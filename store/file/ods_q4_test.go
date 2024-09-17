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
	"github.com/celestiaorg/celestia-node/share/eds"
	"github.com/celestiaorg/celestia-node/share/eds/edstest"
)

func TestCreateODSQ4File(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)

	edsIn := edstest.RandEDS(t, 8)
	odsq4 := createODSQ4File(t, edsIn)

	shares, err := odsq4.Shares(ctx)
	require.NoError(t, err)
	expected := edsIn.FlattenedODS()
	require.Equal(t, expected, shares)
	require.NoError(t, odsq4.Close())
}

func TestODSQ4File(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	t.Cleanup(cancel)

	ODSSize := 16
	eds.TestSuiteAccessor(ctx, t, createODSQ4Accessor, ODSSize)
	eds.TestStreamer(ctx, t, createODSQ4AccessorStreamer, ODSSize)
}

// BenchmarkAxisFromODSQ4File/Size:32/ProofType:row/squareHalf:0-16         	  354836	      3345 ns/op
// BenchmarkAxisFromODSQ4File/Size:32/ProofType:row/squareHalf:1-16         	  339547	      3187 ns/op
// BenchmarkAxisFromODSQ4File/Size:32/ProofType:col/squareHalf:0-16         	   69364	     16440 ns/op
// BenchmarkAxisFromODSQ4File/Size:32/ProofType:col/squareHalf:1-16         	   66928	     15964 ns/op
// BenchmarkAxisFromODSQ4File/Size:64/ProofType:row/squareHalf:0-16         	  223290	      5184 ns/op
// BenchmarkAxisFromODSQ4File/Size:64/ProofType:row/squareHalf:1-16         	  194018	      5240 ns/op
// BenchmarkAxisFromODSQ4File/Size:64/ProofType:col/squareHalf:0-16         	   39949	     29549 ns/op
// BenchmarkAxisFromODSQ4File/Size:64/ProofType:col/squareHalf:1-16         	   39356	     29912 ns/op
// BenchmarkAxisFromODSQ4File/Size:128/ProofType:row/squareHalf:0-16        	  134220	      8903 ns/op
// BenchmarkAxisFromODSQ4File/Size:128/ProofType:row/squareHalf:1-16        	  125110	      8789 ns/op
// BenchmarkAxisFromODSQ4File/Size:128/ProofType:col/squareHalf:0-16        	   15075	     74996 ns/op
// BenchmarkAxisFromODSQ4File/Size:128/ProofType:col/squareHalf:1-16        	   15530	     74855 ns/op
func BenchmarkAxisFromODSQ4File(b *testing.B) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	b.Cleanup(cancel)

	minSize, maxSize := 32, 128
	newFile := func(size int) eds.Accessor {
		eds := edstest.RandEDS(b, size)
		return createODSQ4File(b, eds)
	}
	eds.BenchGetHalfAxisFromAccessor(ctx, b, newFile, minSize, maxSize)
}

// BenchmarkSampleFromODSQ4File/Size:32/quadrant:1-16         	   14260	     82827 ns/op
// BenchmarkSampleFromODSQ4File/Size:32/quadrant:2-16         	   14281	     85465 ns/op
// BenchmarkSampleFromODSQ4File/Size:32/quadrant:3-16         	   12938	     91213 ns/op
// BenchmarkSampleFromODSQ4File/Size:32/quadrant:4-16         	   12934	     94077 ns/op
// BenchmarkSampleFromODSQ4File/Size:64/quadrant:1-16         	    7497	    172978 ns/op
// BenchmarkSampleFromODSQ4File/Size:64/quadrant:2-16         	    6332	    191139 ns/op
// BenchmarkSampleFromODSQ4File/Size:64/quadrant:3-16         	    5852	    214140 ns/op
// BenchmarkSampleFromODSQ4File/Size:64/quadrant:4-16         	    5899	    215875 ns/op
// BenchmarkSampleFromODSQ4File/Size:128/quadrant:1-16        	    3520	    399728 ns/op
// BenchmarkSampleFromODSQ4File/Size:128/quadrant:2-16        	    3242	    410557 ns/op
// BenchmarkSampleFromODSQ4File/Size:128/quadrant:3-16        	    2590	    424491 ns/op
// BenchmarkSampleFromODSQ4File/Size:128/quadrant:4-16        	    2812	    444697 ns/op
func BenchmarkSampleFromODSQ4File(b *testing.B) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	b.Cleanup(cancel)

	minSize, maxSize := 32, 128
	newFile := func(size int) eds.Accessor {
		eds := edstest.RandEDS(b, size)
		return createODSQ4File(b, eds)
	}
	eds.BenchGetSampleFromAccessor(ctx, b, newFile, minSize, maxSize)
}

func createODSQ4AccessorStreamer(t testing.TB, eds *rsmt2d.ExtendedDataSquare) eds.AccessorStreamer {
	return createODSQ4File(t, eds)
}

func createODSQ4Accessor(t testing.TB, eds *rsmt2d.ExtendedDataSquare) eds.Accessor {
	return createODSQ4File(t, eds)
}

func createODSQ4File(t testing.TB, eds *rsmt2d.ExtendedDataSquare) *ODSQ4 {
	path := t.TempDir() + "/" + strconv.Itoa(rand.Intn(1000))
	roots, err := share.NewAxisRoots(eds)
	require.NoError(t, err)
	pathODS, pathQ4 := path+".ods", path+".q4"
	err = CreateODSQ4(pathODS, pathQ4, roots, eds)
	require.NoError(t, err)
	ods, err := OpenODS(pathODS)
	require.NoError(t, err)
	return ODSWithQ4(ods, pathQ4)
}
