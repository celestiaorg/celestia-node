package eds

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	ds "github.com/ipfs/go-datastore"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	carv1 "github.com/ipld/go-car"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-app/pkg/appconsts"
	"github.com/celestiaorg/celestia-app/pkg/da"
	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/ipld"
)

//go:embed "testdata/example-root.json"
var exampleRoot string

//go:embed "testdata/example.car"
var f embed.FS

func TestQuadrantOrder(t *testing.T) {
	// TODO: add more test cases
	nID := []byte{0, 0, 0, 0, 0, 0, 0, 0}
	parity := append(appconsts.ParitySharesNamespaceID, nID...) //nolint
	doubleNID := append(nID, nID...)                            //nolint
	result, _ := rsmt2d.ComputeExtendedDataSquare([][]byte{
		append(nID, 1), append(nID, 2),
		append(nID, 3), append(nID, 4),
	}, rsmt2d.NewRSGF8Codec(), rsmt2d.NewDefaultTree)
	//  {{1}, {2}, {7}, {13}},
	//  {{3}, {4}, {13}, {31}},
	//  {{5}, {14}, {19}, {41}},
	//  {{9}, {26}, {47}, {69}},
	require.Equal(t,
		[][]byte{
			append(doubleNID, 1), append(doubleNID, 2), append(doubleNID, 3), append(doubleNID, 4),
			append(parity, 7), append(parity, 13), append(parity, 13), append(parity, 31),
			append(parity, 5), append(parity, 14), append(parity, 9), append(parity, 26),
			append(parity, 19), append(parity, 41), append(parity, 47), append(parity, 69),
		}, quadrantOrder(result),
	)
}

func TestWriteEDS(t *testing.T) {
	writeRandomEDS(t)
}

func TestWriteEDSHeaderRoots(t *testing.T) {
	eds := writeRandomEDS(t)
	f := openWrittenEDS(t)
	defer f.Close()

	reader, err := carv1.NewCarReader(f)
	require.NoError(t, err, "error creating car reader")
	roots, err := rootsToCids(eds)
	require.NoError(t, err, "error converting roots to cids")
	require.Equal(t, roots, reader.Header.Roots)
}

func TestWriteEDSStartsWithLeaves(t *testing.T) {
	eds := writeRandomEDS(t)
	f := openWrittenEDS(t)
	defer f.Close()

	reader, err := carv1.NewCarReader(f)
	require.NoError(t, err, "error creating car reader")
	block, err := reader.Next()
	require.NoError(t, err, "error getting first block")

	require.Equal(t, block.RawData()[ipld.NamespaceSize:], eds.GetCell(0, 0))
}

func TestWriteEDSIncludesRoots(t *testing.T) {
	writeRandomEDS(t)
	f := openWrittenEDS(t)
	defer f.Close()

	bs := blockstore.NewBlockstore(ds.NewMapDatastore())
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	loaded, err := carv1.LoadCar(ctx, bs, f)
	require.NoError(t, err, "error loading car file")
	for _, root := range loaded.Roots {
		ok, err := bs.Has(context.Background(), root)
		require.NoError(t, err, "error checking if blockstore has root")
		require.True(t, ok, "blockstore does not have root")
	}
}

func TestWriteEDSInQuadrantOrder(t *testing.T) {
	eds := writeRandomEDS(t)
	f := openWrittenEDS(t)
	defer f.Close()

	reader, err := carv1.NewCarReader(f)
	require.NoError(t, err, "error creating car reader")

	shares := quadrantOrder(eds)
	for i := 0; i < len(shares); i++ {
		block, err := reader.Next()
		require.NoError(t, err, "error getting block")
		require.Equal(t, block.RawData(), shares[i])
	}
}

// TestInnerNodeBatchSize verifies that the number of unique inner nodes is equal to ipld.BatchSize - shareCount.
func TestInnerNodeBatchSize(t *testing.T) {
	tests := []struct {
		name      string
		origWidth int
	}{
		{"2", 2},
		{"4", 4},
		{"8", 8},
		{"16", 16},
		{"32", 32},
		// {"64", 64}, // test case too large for CI with race detector
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			extendedWidth := tt.origWidth * 2
			shareCount := extendedWidth * extendedWidth
			assert.Equalf(
				t,
				innerNodeBatchSize(shareCount, tt.origWidth),
				ipld.BatchSize(extendedWidth)-shareCount,
				"batchSize(%v)", extendedWidth,
			)
		})
	}
}

func TestReadWriteRoundtrip(t *testing.T) {
	eds := writeRandomEDS(t)
	dah := da.NewDataAvailabilityHeader(eds)
	f := openWrittenEDS(t)
	defer f.Close()

	loaded, err := ReadEDS(context.Background(), f, dah)
	require.NoError(t, err, "error reading EDS from file")
	require.Equal(t, eds.RowRoots(), loaded.RowRoots())
	require.Equal(t, eds.ColRoots(), loaded.ColRoots())
}

func TestReadEDS(t *testing.T) {
	f, err := f.Open("testdata/example.car")
	require.NoError(t, err, "error opening file")

	var dah da.DataAvailabilityHeader
	err = json.Unmarshal([]byte(exampleRoot), &dah)
	require.NoError(t, err, "error unmarshaling example root")

	loaded, err := ReadEDS(context.Background(), f, dah)
	require.NoError(t, err, "error reading EDS from file")
	require.Equal(t, dah.RowsRoots, loaded.RowRoots())
	require.Equal(t, dah.ColumnRoots, loaded.ColRoots())
}

func TestReadEDSContentIntegrityMismatch(t *testing.T) {
	writeRandomEDS(t)
	dah := da.NewDataAvailabilityHeader(share.RandEDS(t, 4))
	f := openWrittenEDS(t)
	defer f.Close()

	_, err := ReadEDS(context.Background(), f, dah)
	require.ErrorContains(t, err, "share: content integrity mismatch: imported root")
}

func BenchmarkReadWriteEDS(b *testing.B) {
	ctx, cancel := context.WithCancel(context.Background())
	b.Cleanup(cancel)
	for originalDataWidth := 4; originalDataWidth <= 64; originalDataWidth *= 2 {
		tmpDir := b.TempDir()
		err := os.Chdir(tmpDir)
		require.NoError(b, err)
		eds := share.RandBenchmarkEDS(b, originalDataWidth)
		dah := da.NewDataAvailabilityHeader(eds)
		b.Run(fmt.Sprintf("Writing %dx%d", originalDataWidth, originalDataWidth), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				f, _ := os.OpenFile(fmt.Sprintf("%d.car", originalDataWidth), os.O_WRONLY|os.O_CREATE, 0600)
				err := WriteEDS(ctx, eds, f)
				require.NoError(b, err)
				f.Close()
			}
		})
		b.Run(fmt.Sprintf("Reading %dx%d", originalDataWidth, originalDataWidth), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				f, _ := os.OpenFile(fmt.Sprintf("%d.car", originalDataWidth), os.O_RDONLY, 0600)
				_, err = ReadEDS(ctx, f, dah)
				require.NoError(b, err)
				f.Close()
			}
		})
	}
}

func writeRandomEDS(t *testing.T) *rsmt2d.ExtendedDataSquare {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	tmpDir := t.TempDir()
	err := os.Chdir(tmpDir)
	require.NoError(t, err, "error changing to the temporary test directory")
	f, err := os.OpenFile("test.car", os.O_WRONLY|os.O_CREATE, 0600)
	require.NoError(t, err, "error opening file")

	eds := share.RandEDS(t, 4)
	err = WriteEDS(ctx, eds, f)
	require.NoError(t, err, "error writing EDS to file")
	f.Close()
	return eds
}

func openWrittenEDS(t *testing.T) *os.File {
	t.Helper()
	f, err := os.OpenFile("test.car", os.O_RDONLY, 0600)
	require.NoError(t, err, "error opening file")
	return f
}
