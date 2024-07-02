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
	datahash := share.DataHash(rand.Bytes(32))
	path := t.TempDir() + "/" + datahash.String()
	f, err := CreateODSFile(path, datahash, edsIn)
	require.NoError(t, err)

	shares, err := f.Shares(ctx)
	require.NoError(t, err)
	expected := edsIn.FlattenedODS()
	require.Equal(t, expected, shares)
	require.Equal(t, datahash, f.hdr.datahash)
	require.NoError(t, f.Close())

	f, err = OpenODSFile(path)
	require.NoError(t, err)
	shares, err = f.Shares(ctx)
	require.NoError(t, err)
	require.Equal(t, expected, shares)
	require.Equal(t, datahash, f.hdr.datahash)
	require.NoError(t, f.Close())
}

func TestReadODSFromFile(t *testing.T) {
	eds := edstest.RandEDS(t, 8)
	path := t.TempDir() + "/testfile"
	f, err := CreateODSFile(path, []byte{}, eds)
	require.NoError(t, err)

	ods, err := f.readODS()
	require.NoError(t, err)
	for i, row := range ods {
		original := eds.Row(uint(i))[:eds.Width()/2]
		require.True(t, len(original) == len(row))
		require.Equal(t, original, row)
	}
}

func TestODSFile(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)

	ODSSize := 64
	eds.TestSuiteAccessor(ctx, t, createAccessor, ODSSize)
	eds.TestStreamer(ctx, t, createCachedStreamer, ODSSize)
	eds.TestStreamer(ctx, t, createStreamer, ODSSize)
}

// BenchmarkAxisFromODSFile/Size:32/ProofType:row/squareHalf:0-10         	  460231	      2555 ns/op
// BenchmarkAxisFromODSFile/Size:32/ProofType:row/squareHalf:1-10         	    5320	    218609 ns/op
// BenchmarkAxisFromODSFile/Size:32/ProofType:col/squareHalf:0-10         	 4572247	       256.7 ns/op
// BenchmarkAxisFromODSFile/Size:32/ProofType:col/squareHalf:1-10         	    5170	    212567 ns/op
// BenchmarkAxisFromODSFile/Size:64/ProofType:row/squareHalf:0-10         	  299281	      3777 ns/op
// BenchmarkAxisFromODSFile/Size:64/ProofType:row/squareHalf:1-10         	    1646	    661930 ns/op
// BenchmarkAxisFromODSFile/Size:64/ProofType:col/squareHalf:0-10         	 3318733	       359.1 ns/op
// BenchmarkAxisFromODSFile/Size:64/ProofType:col/squareHalf:1-10         	    1600	    648482 ns/op
// BenchmarkAxisFromODSFile/Size:128/ProofType:row/squareHalf:0-10        	  170642	      6347 ns/op
// BenchmarkAxisFromODSFile/Size:128/ProofType:row/squareHalf:1-10        	     328	   3194674 ns/op
// BenchmarkAxisFromODSFile/Size:128/ProofType:col/squareHalf:0-10        	 1931910	       640.9 ns/op
// BenchmarkAxisFromODSFile/Size:128/ProofType:col/squareHalf:1-10        	     387	   3304090 ns/op
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

// BenchmarkAxisFromODSFileDisabledCache/Size:32/ProofType:row/squareHalf:0-10         	  481326	      2447 ns/op
// BenchmarkAxisFromODSFileDisabledCache/Size:32/ProofType:row/squareHalf:1-10         	    5134	    218191 ns/op
// BenchmarkAxisFromODSFileDisabledCache/Size:32/ProofType:col/squareHalf:0-10         	   56260	     21109 ns/op
// BenchmarkAxisFromODSFileDisabledCache/Size:32/ProofType:col/squareHalf:1-10         	    5608	    217877 ns/op
// BenchmarkAxisFromODSFileDisabledCache/Size:64/ProofType:row/squareHalf:0-10         	  321994	      3941 ns/op
// BenchmarkAxisFromODSFileDisabledCache/Size:64/ProofType:row/squareHalf:1-10         	    1237	    919419 ns/op
// BenchmarkAxisFromODSFileDisabledCache/Size:64/ProofType:col/squareHalf:0-10         	   28233	     43209 ns/op
// BenchmarkAxisFromODSFileDisabledCache/Size:64/ProofType:col/squareHalf:1-10         	    1334	    898654 ns/op
// BenchmarkAxisFromODSFileDisabledCache/Size:128/ProofType:row/squareHalf:0-10        	  179788	      6839 ns/op
// BenchmarkAxisFromODSFileDisabledCache/Size:128/ProofType:row/squareHalf:1-10        	     310	   3935097 ns/op
// BenchmarkAxisFromODSFileDisabledCache/Size:128/ProofType:col/squareHalf:0-10        	   13867	     85854 ns/op
// BenchmarkAxisFromODSFileDisabledCache/Size:128/ProofType:col/squareHalf:1-10        	     298	   3900021 ns/op
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

// BenchmarkSampleFromODSFile/Size:32/quadrant:1-10         	   10908	    104872 ns/op
// BenchmarkSampleFromODSFile/Size:32/quadrant:2-10         	    9906	    104641 ns/op
// BenchmarkSampleFromODSFile/Size:32/quadrant:3-10         	    8983	    123384 ns/op
// BenchmarkSampleFromODSFile/Size:32/quadrant:4-10         	    3476	    343850 ns/op
// BenchmarkSampleFromODSFile/Size:64/quadrant:1-10         	    5835	    200151 ns/op
// BenchmarkSampleFromODSFile/Size:64/quadrant:2-10         	    5401	    201271 ns/op
// BenchmarkSampleFromODSFile/Size:64/quadrant:3-10         	    4648	    239045 ns/op
// BenchmarkSampleFromODSFile/Size:64/quadrant:4-10         	    1263	    895983 ns/op
// BenchmarkSampleFromODSFile/Size:128/quadrant:1-10        	    2475	    409687 ns/op
// BenchmarkSampleFromODSFile/Size:128/quadrant:2-10        	    2790	    411153 ns/op
// BenchmarkSampleFromODSFile/Size:128/quadrant:3-10        	    2286	    487123 ns/op
// BenchmarkSampleFromODSFile/Size:128/quadrant:4-10        	     321	   3698735 ns/op
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

// BenchmarkSampleFromODSFileDisabledCache/Size:32/quadrant:1-10         	   11040	    106378 ns/op
// BenchmarkSampleFromODSFileDisabledCache/Size:32/quadrant:2-10         	    9936	    106403 ns/op
// BenchmarkSampleFromODSFileDisabledCache/Size:32/quadrant:3-10         	    8635	    124142 ns/op
// BenchmarkSampleFromODSFileDisabledCache/Size:32/quadrant:4-10         	    1940	    596330 ns/op
// BenchmarkSampleFromODSFileDisabledCache/Size:64/quadrant:1-10         	    5930	    199782 ns/op
// BenchmarkSampleFromODSFileDisabledCache/Size:64/quadrant:2-10         	    5494	    201658 ns/op
// BenchmarkSampleFromODSFileDisabledCache/Size:64/quadrant:3-10         	    4756	    237897 ns/op
// BenchmarkSampleFromODSFileDisabledCache/Size:64/quadrant:4-10         	     638	   1874038 ns/op
// BenchmarkSampleFromODSFileDisabledCache/Size:128/quadrant:1-10        	    2500	    408092 ns/op
// BenchmarkSampleFromODSFileDisabledCache/Size:128/quadrant:2-10        	    2696	    410861 ns/op
// BenchmarkSampleFromODSFileDisabledCache/Size:128/quadrant:3-10        	    2290	    490488 ns/op
// BenchmarkSampleFromODSFileDisabledCache/Size:128/quadrant:4-10        	     159	   7660843 ns/op
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

func createODSFile(t testing.TB, eds *rsmt2d.ExtendedDataSquare) *ODSFile {
	path := t.TempDir() + "/" + strconv.Itoa(rand.Intn(1000))
	fl, err := CreateODSFile(path, []byte{}, eds)
	require.NoError(t, err)
	return fl
}

func createODSFileDisabledCache(t testing.TB, eds *rsmt2d.ExtendedDataSquare) eds.Accessor {
	path := t.TempDir() + "/" + strconv.Itoa(rand.Intn(1000))
	fl, err := CreateODSFile(path, []byte{}, eds)
	require.NoError(t, err)
	fl.disableCache = true
	return fl
}
