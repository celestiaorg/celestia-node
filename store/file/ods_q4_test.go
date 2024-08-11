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
	eds.TestSuiteAccessor(ctx, t, createODSAccessor, ODSSize)
	eds.TestStreamer(ctx, t, createODSAccessorStreamer, ODSSize)
}

// BenchmarkAxisFromODSQ4File/Size:32/ProofType:row/squareHalf:0-10         	  481144	      2413 ns/op
// BenchmarkAxisFromODSQ4File/Size:32/ProofType:row/squareHalf:1-10         	  479437	      2431 ns/op
// BenchmarkAxisFromODSQ4File/Size:32/ProofType:col/squareHalf:0-10         	   56775	     21272 ns/op
// BenchmarkAxisFromODSQ4File/Size:32/ProofType:col/squareHalf:1-10         	   57283	     20941 ns/op
// BenchmarkAxisFromODSQ4File/Size:64/ProofType:row/squareHalf:0-10         	  301357	      3870 ns/op
// BenchmarkAxisFromODSQ4File/Size:64/ProofType:row/squareHalf:1-10         	  329796	      3913 ns/op
// BenchmarkAxisFromODSQ4File/Size:64/ProofType:col/squareHalf:0-10         	   28035	     42560 ns/op
// BenchmarkAxisFromODSQ4File/Size:64/ProofType:col/squareHalf:1-10         	   28179	     42447 ns/op
// BenchmarkAxisFromODSQ4File/Size:128/ProofType:row/squareHalf:0-10        	  170607	      6368 ns/op
// BenchmarkAxisFromODSQ4File/Size:128/ProofType:row/squareHalf:1-10        	  184579	      6517 ns/op
// BenchmarkAxisFromODSQ4File/Size:128/ProofType:col/squareHalf:0-10        	   13952	     84004 ns/op
// BenchmarkAxisFromODSQ4File/Size:128/ProofType:col/squareHalf:1-10        	   14398	     83240 ns/op
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

// BenchmarkSampleFromODSQ4File/Size:32/quadrant:1-10         	   11000	    103665 ns/op
// BenchmarkSampleFromODSQ4File/Size:32/quadrant:2-10         	    9757	    105395 ns/op
// BenchmarkSampleFromODSQ4File/Size:32/quadrant:3-10         	    8880	    119081 ns/op
// BenchmarkSampleFromODSQ4File/Size:32/quadrant:4-10         	    9049	    118572 ns/op
// BenchmarkSampleFromODSQ4File/Size:64/quadrant:1-10         	    5372	    200685 ns/op
// BenchmarkSampleFromODSQ4File/Size:64/quadrant:2-10         	    5499	    200007 ns/op
// BenchmarkSampleFromODSQ4File/Size:64/quadrant:3-10         	    4879	    233044 ns/op
// BenchmarkSampleFromODSQ4File/Size:64/quadrant:4-10         	    4802	    232696 ns/op
// BenchmarkSampleFromODSQ4File/Size:128/quadrant:1-10        	    2810	    398169 ns/op
// BenchmarkSampleFromODSQ4File/Size:128/quadrant:2-10        	    2830	    407077 ns/op
// BenchmarkSampleFromODSQ4File/Size:128/quadrant:3-10        	    2434	    478656 ns/op
// BenchmarkSampleFromODSQ4File/Size:128/quadrant:4-10        	    2368	    478021 ns/op
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

func createODSAccessorStreamer(t testing.TB, eds *rsmt2d.ExtendedDataSquare) eds.AccessorStreamer {
	return createODSFile(t, eds)
}

func createODSAccessor(t testing.TB, eds *rsmt2d.ExtendedDataSquare) eds.Accessor {
	return createODSFile(t, eds)
}

func createODSQ4File(t testing.TB, eds *rsmt2d.ExtendedDataSquare) *ODSQ4 {
	path := t.TempDir() + "/" + strconv.Itoa(rand.Intn(1000))
	roots, err := share.NewAxisRoots(eds)
	require.NoError(t, err)
	pathODS, pathQ4 := path+".ods", path+".q4"
	err = CreateODSQ4(pathODS, pathQ4, roots, eds)
	require.NoError(t, err)
	odsq4, err := OpenODSQ4(pathODS, pathQ4)
	require.NoError(t, err)
	return odsq4
}
