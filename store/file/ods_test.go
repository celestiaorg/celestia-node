package file

import (
	"context"
	"fmt"
	"io"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/cometbft/cometbft/libs/rand"
	"github.com/stretchr/testify/require"

	libshare "github.com/celestiaorg/go-square/v3/share"
	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds"
	"github.com/celestiaorg/celestia-node/share/eds/edstest"
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
	require.Equal(t, expected, libshare.ToBytes(shares))

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
		require.Equal(t, original, libshare.ToBytes(row))
	}
}

func TestODSFile(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	t.Cleanup(cancel)

	ODSSize := 16
	eds.TestSuiteAccessor(ctx, t, createODSAccessor, ODSSize)
	eds.TestStreamer(ctx, t, createCachedStreamer, ODSSize)
	eds.TestStreamer(ctx, t, createODSAccessorStreamer, ODSSize)
}

func TestValidateODSSize(t *testing.T) {
	edses := []struct {
		name string
		eds  *rsmt2d.ExtendedDataSquare
	}{
		{
			name: "no padding",
			eds:  edstest.RandEDS(t, 8),
		},
		{
			name: "with padding",
			eds:  edstest.RandEDSWithTailPadding(t, 8, 11),
		},
		{
			name: "empty", eds: share.EmptyEDS(),
		},
	}

	tests := []struct {
		name       string
		createFile func(path string, roots *share.AxisRoots, eds *rsmt2d.ExtendedDataSquare) error
		valid      bool
	}{
		{
			name: "valid",
			createFile: func(path string, roots *share.AxisRoots, eds *rsmt2d.ExtendedDataSquare) error {
				return CreateODS(path, roots, eds)
			},
			valid: true,
		},
		{
			name: "shorter",
			createFile: func(path string, roots *share.AxisRoots, eds *rsmt2d.ExtendedDataSquare) error {
				err := CreateODS(path, roots, eds)
				if err != nil {
					return err
				}
				file, err := os.OpenFile(path, os.O_RDWR, 0)
				if err != nil {
					return err
				}
				defer file.Close()
				info, err := file.Stat()
				if err != nil {
					return err
				}
				return file.Truncate(info.Size() - 1)
			},
			valid: false,
		},
		{
			name: "longer",
			createFile: func(path string, roots *share.AxisRoots, eds *rsmt2d.ExtendedDataSquare) error {
				err := CreateODS(path, roots, eds)
				if err != nil {
					return err
				}
				file, err := os.OpenFile(path, os.O_RDWR, 0)
				if err != nil {
					return err
				}
				defer file.Close()
				// append 1 byte to the file
				_, err = file.Seek(0, io.SeekEnd)
				if err != nil {
					return err
				}
				_, err = file.Write([]byte{0})
				return err
			},
			valid: false,
		},
	}

	for _, tt := range tests {
		for _, eds := range edses {
			t.Run(fmt.Sprintf("%s/%s", tt.name, eds.name), func(t *testing.T) {
				path := t.TempDir() + tt.name + eds.name
				roots, err := share.NewAxisRoots(eds.eds)
				require.NoError(t, err)
				err = tt.createFile(path, roots, eds.eds)
				require.NoError(t, err)

				err = ValidateODSSize(path, eds.eds)
				require.Equal(t, tt.valid, err == nil)
			})
		}
	}
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
	eds.BenchGetHalfAxisFromAccessor(ctx, b, createODSAccessor, minSize, maxSize)
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
	eds.BenchGetHalfAxisFromAccessor(ctx, b, createODSFileDisabledCache, minSize, maxSize)
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
	eds.BenchGetSampleFromAccessor(ctx, b, createODSAccessor, minSize, maxSize)
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
	eds.BenchGetSampleFromAccessor(ctx, b, createODSFileDisabledCache, minSize, maxSize)
}

func createODSAccessorStreamer(t testing.TB, eds *rsmt2d.ExtendedDataSquare) eds.AccessorStreamer {
	return createODSFile(t, eds)
}

func createODSAccessor(t testing.TB, eds *rsmt2d.ExtendedDataSquare) eds.Accessor {
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
	require.NoError(t, err)
	return ods
}

func createODSFileDisabledCache(t testing.TB, eds *rsmt2d.ExtendedDataSquare) eds.Accessor {
	ods := createODSFile(t, eds)
	ods.disableCache = true
	return ods
}
