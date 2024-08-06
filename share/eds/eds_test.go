package eds

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	bstore "github.com/ipfs/boxo/blockstore"
	ds "github.com/ipfs/go-datastore"
	carv1 "github.com/ipld/go-car"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/libs/rand"
	coretypes "github.com/tendermint/tendermint/types"

	"github.com/celestiaorg/celestia-app/pkg/appconsts"
	"github.com/celestiaorg/celestia-app/pkg/da"
	"github.com/celestiaorg/celestia-app/pkg/namespace"
	pkgproof "github.com/celestiaorg/celestia-app/pkg/proof"
	"github.com/celestiaorg/celestia-app/pkg/shares"
	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds/edstest"
)

//go:embed "testdata/example-root.json"
var exampleRoot string

//go:embed "testdata/example.car"
var f embed.FS

func TestQuadrantOrder(t *testing.T) {
	testCases := []struct {
		name       string
		squareSize int
	}{
		{"smol", 2},
		{"still smol", 8},
		{"default mainnet", appconsts.DefaultGovMaxSquareSize},
		{"max", share.MaxSquareSize},
	}

	testShareSize := 64

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			shares := make([][]byte, tc.squareSize*tc.squareSize)

			for i := 0; i < tc.squareSize*tc.squareSize; i++ {
				shares[i] = rand.Bytes(testShareSize)
			}

			eds, err := rsmt2d.ComputeExtendedDataSquare(shares, share.DefaultRSMT2DCodec(), rsmt2d.NewDefaultTree)
			require.NoError(t, err)

			res := quadrantOrder(eds)
			for _, s := range res {
				require.Len(t, s, testShareSize+share.NamespaceSize)
			}

			for q := 0; q < 4; q++ {
				for i := 0; i < tc.squareSize; i++ {
					for j := 0; j < tc.squareSize; j++ {
						resIndex := q*tc.squareSize*tc.squareSize + i*tc.squareSize + j
						edsRow := q/2*tc.squareSize + i
						edsCol := (q%2)*tc.squareSize + j

						assert.Equal(t, res[resIndex], prependNamespace(q, eds.Row(uint(edsRow))[edsCol]))
					}
				}
			}
		})
	}
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

	require.Equal(t, share.GetData(block.RawData()), eds.GetCell(0, 0))
}

func TestWriteEDSIncludesRoots(t *testing.T) {
	writeRandomEDS(t)
	f := openWrittenEDS(t)
	defer f.Close()

	bs := bstore.NewBlockstore(ds.NewMapDatastore())
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

func TestReadWriteRoundtrip(t *testing.T) {
	eds := writeRandomEDS(t)
	dah, err := share.NewRoot(eds)
	require.NoError(t, err)
	f := openWrittenEDS(t)
	defer f.Close()

	loaded, err := ReadEDS(context.Background(), f, dah.Hash())
	require.NoError(t, err, "error reading EDS from file")

	rowRoots, err := eds.RowRoots()
	require.NoError(t, err)
	loadedRowRoots, err := loaded.RowRoots()
	require.NoError(t, err)
	require.Equal(t, rowRoots, loadedRowRoots)

	colRoots, err := eds.ColRoots()
	require.NoError(t, err)
	loadedColRoots, err := loaded.ColRoots()
	require.NoError(t, err)
	require.Equal(t, colRoots, loadedColRoots)
}

func TestReadEDS(t *testing.T) {
	f, err := f.Open("testdata/example.car")
	require.NoError(t, err, "error opening file")

	var dah da.DataAvailabilityHeader
	err = json.Unmarshal([]byte(exampleRoot), &dah)
	require.NoError(t, err, "error unmarshaling example root")

	loaded, err := ReadEDS(context.Background(), f, dah.Hash())
	require.NoError(t, err, "error reading EDS from file")
	rowRoots, err := loaded.RowRoots()
	require.NoError(t, err)
	require.Equal(t, dah.RowRoots, rowRoots)
	colRoots, err := loaded.ColRoots()
	require.NoError(t, err)
	require.Equal(t, dah.ColumnRoots, colRoots)
}

func TestReadEDSContentIntegrityMismatch(t *testing.T) {
	writeRandomEDS(t)
	dah, err := da.NewDataAvailabilityHeader(edstest.RandEDS(t, 4))
	require.NoError(t, err)
	f := openWrittenEDS(t)
	defer f.Close()

	_, err = ReadEDS(context.Background(), f, dah.Hash())
	require.ErrorContains(t, err, "share: content integrity mismatch: imported root")
}

// BenchmarkReadWriteEDS benchmarks the time it takes to write and read an EDS from disk. The
// benchmark is run with a 4x4 ODS to a 64x64 ODS - a higher value can be used, but it will run for
// much longer.
func BenchmarkReadWriteEDS(b *testing.B) {
	ctx, cancel := context.WithCancel(context.Background())
	b.Cleanup(cancel)
	for originalDataWidth := 4; originalDataWidth <= 64; originalDataWidth *= 2 {
		eds := edstest.RandEDS(b, originalDataWidth)
		dah, err := share.NewRoot(eds)
		require.NoError(b, err)
		b.Run(fmt.Sprintf("Writing %dx%d", originalDataWidth, originalDataWidth), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				f := new(bytes.Buffer)
				err := WriteEDS(ctx, eds, f)
				require.NoError(b, err)
			}
		})
		b.Run(fmt.Sprintf("Reading %dx%d", originalDataWidth, originalDataWidth), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				b.StopTimer()
				f := new(bytes.Buffer)
				_ = WriteEDS(ctx, eds, f)
				b.StartTimer()
				_, err := ReadEDS(ctx, f, dah.Hash())
				require.NoError(b, err)
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
	f, err := os.OpenFile("test.car", os.O_WRONLY|os.O_CREATE, 0o600)
	require.NoError(t, err, "error opening file")

	eds := edstest.RandEDS(t, 4)
	err = WriteEDS(ctx, eds, f)
	require.NoError(t, err, "error writing EDS to file")
	f.Close()
	return eds
}

func openWrittenEDS(t *testing.T) *os.File {
	t.Helper()
	f, err := os.OpenFile("test.car", os.O_RDONLY, 0o600)
	require.NoError(t, err, "error opening file")
	return f
}

/*
use this function as needed to create new test data.

example:

	func Test_CreateData(t *testing.T) {
		createTestData(t, "celestia-node/share/eds/testdata")
	}
*/
func createTestData(t *testing.T, testDir string) { //nolint:unused
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	err := os.Chdir(testDir)
	require.NoError(t, err, "changing to the directory")
	os.RemoveAll("example.car")
	require.NoError(t, err, "removing old file")
	f, err := os.OpenFile("example.car", os.O_WRONLY|os.O_CREATE, 0o600)
	require.NoError(t, err, "opening file")

	eds := edstest.RandEDS(t, 4)
	err = WriteEDS(ctx, eds, f)
	require.NoError(t, err, "writing EDS to file")
	f.Close()
	dah, err := share.NewRoot(eds)
	require.NoError(t, err)

	header, err := json.MarshalIndent(dah, "", "")
	require.NoError(t, err, "marshaling example root")
	os.RemoveAll("example-root.json")
	require.NoError(t, err, "removing old file")
	f, err = os.OpenFile("example-root.json", os.O_WRONLY|os.O_CREATE, 0o600)
	require.NoError(t, err, "opening file")
	_, err = f.Write(header)
	require.NoError(t, err, "writing example root to file")
	f.Close()
}

func TestProveShares(t *testing.T) {
	ns := namespace.RandomBlobNamespace()
	eds, dataRoot := edstest.RandEDSWithNamespace(
		t,
		ns.Bytes(),
		16,
	)

	tests := map[string]struct {
		start, end    int
		expectedProof coretypes.ShareProof
		expectErr     bool
	}{
		"start share == end share": {
			start:     2,
			end:       2,
			expectErr: true,
		},
		"start share > end share": {
			start:     3,
			end:       2,
			expectErr: true,
		},
		"start share > number of shares in the block": {
			start:     2000,
			end:       2010,
			expectErr: true,
		},
		"end share > number of shares in the block": {
			start:     1,
			end:       2010,
			expectErr: true,
		},
		"valid case": {
			start: 0,
			end:   2,
			expectedProof: func() coretypes.ShareProof {
				proof, err := pkgproof.NewShareInclusionProofFromEDS(
					eds,
					ns,
					shares.NewRange(0, 2),
				)
				require.NoError(t, err)
				require.NoError(t, proof.Validate(dataRoot.Hash()))
				return proof
			}(),
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			result, err := ProveShares(eds, tc.start, tc.end)
			if tc.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expectedProof, *result)
				assert.NoError(t, result.Validate(dataRoot.Hash()))
			}
		})
	}
}
