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

func TestCreateQ1Q4File(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)

	edsIn := edstest.RandEDS(t, 8)
	roots, err := share.NewAxisRoots(edsIn)
	require.NoError(t, err)
	path := t.TempDir() + "/" + roots.String()
	f, err := CreateQ1Q4File(path, roots, edsIn)
	require.NoError(t, err)

	shares, err := f.Shares(ctx)
	require.NoError(t, err)
	expected := edsIn.FlattenedODS()
	require.Equal(t, expected, shares)
	require.NoError(t, f.Close())

	f, err = OpenQ1Q4File(path)
	require.NoError(t, err)
	shares, err = f.Shares(ctx)
	require.NoError(t, err)
	require.Equal(t, expected, shares)
	require.NoError(t, f.Close())
}

func TestQ1Q4File(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	t.Cleanup(cancel)

	ODSSize := 16
	eds.TestSuiteAccessor(ctx, t, createQ1Q4File, ODSSize)
}

// BenchmarkAxisFromQ1Q4File/Size:32/ProofType:row/squareHalf:0-10         	  481144	      2413 ns/op
// BenchmarkAxisFromQ1Q4File/Size:32/ProofType:row/squareHalf:1-10         	  479437	      2431 ns/op
// BenchmarkAxisFromQ1Q4File/Size:32/ProofType:col/squareHalf:0-10         	   56775	     21272 ns/op
// BenchmarkAxisFromQ1Q4File/Size:32/ProofType:col/squareHalf:1-10         	   57283	     20941 ns/op
// BenchmarkAxisFromQ1Q4File/Size:64/ProofType:row/squareHalf:0-10         	  301357	      3870 ns/op
// BenchmarkAxisFromQ1Q4File/Size:64/ProofType:row/squareHalf:1-10         	  329796	      3913 ns/op
// BenchmarkAxisFromQ1Q4File/Size:64/ProofType:col/squareHalf:0-10         	   28035	     42560 ns/op
// BenchmarkAxisFromQ1Q4File/Size:64/ProofType:col/squareHalf:1-10         	   28179	     42447 ns/op
// BenchmarkAxisFromQ1Q4File/Size:128/ProofType:row/squareHalf:0-10        	  170607	      6368 ns/op
// BenchmarkAxisFromQ1Q4File/Size:128/ProofType:row/squareHalf:1-10        	  184579	      6517 ns/op
// BenchmarkAxisFromQ1Q4File/Size:128/ProofType:col/squareHalf:0-10        	   13952	     84004 ns/op
// BenchmarkAxisFromQ1Q4File/Size:128/ProofType:col/squareHalf:1-10        	   14398	     83240 ns/op
func BenchmarkAxisFromQ1Q4File(b *testing.B) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	b.Cleanup(cancel)

	minSize, maxSize := 32, 128
	newFile := func(size int) eds.Accessor {
		eds := edstest.RandEDS(b, size)
		return createQ1Q4File(b, eds)
	}
	eds.BenchGetHalfAxisFromAccessor(ctx, b, newFile, minSize, maxSize)
}

// BenchmarkSampleFromQ1Q4File/Size:32/quadrant:1-10         	   11000	    103665 ns/op
// BenchmarkSampleFromQ1Q4File/Size:32/quadrant:2-10         	    9757	    105395 ns/op
// BenchmarkSampleFromQ1Q4File/Size:32/quadrant:3-10         	    8880	    119081 ns/op
// BenchmarkSampleFromQ1Q4File/Size:32/quadrant:4-10         	    9049	    118572 ns/op
// BenchmarkSampleFromQ1Q4File/Size:64/quadrant:1-10         	    5372	    200685 ns/op
// BenchmarkSampleFromQ1Q4File/Size:64/quadrant:2-10         	    5499	    200007 ns/op
// BenchmarkSampleFromQ1Q4File/Size:64/quadrant:3-10         	    4879	    233044 ns/op
// BenchmarkSampleFromQ1Q4File/Size:64/quadrant:4-10         	    4802	    232696 ns/op
// BenchmarkSampleFromQ1Q4File/Size:128/quadrant:1-10        	    2810	    398169 ns/op
// BenchmarkSampleFromQ1Q4File/Size:128/quadrant:2-10        	    2830	    407077 ns/op
// BenchmarkSampleFromQ1Q4File/Size:128/quadrant:3-10        	    2434	    478656 ns/op
// BenchmarkSampleFromQ1Q4File/Size:128/quadrant:4-10        	    2368	    478021 ns/op
func BenchmarkSampleFromQ1Q4File(b *testing.B) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	b.Cleanup(cancel)

	minSize, maxSize := 32, 128
	newFile := func(size int) eds.Accessor {
		eds := edstest.RandEDS(b, size)
		return createQ1Q4File(b, eds)
	}
	eds.BenchGetSampleFromAccessor(ctx, b, newFile, minSize, maxSize)
}

func createQ1Q4File(t testing.TB, eds *rsmt2d.ExtendedDataSquare) eds.Accessor {
	path := t.TempDir() + "/" + strconv.Itoa(rand.Intn(1000))
	roots, err := share.NewAxisRoots(eds)
	require.NoError(t, err)
	fl, err := CreateQ1Q4File(path, roots, eds)
	require.NoError(t, err)
	return fl
}
