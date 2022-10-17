package eds

import (
	"context"
	"os"
	"testing"

	ds "github.com/ipfs/go-datastore"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	carv1 "github.com/ipld/go-car"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-app/pkg/appconsts"
	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/ipld"
)

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
	loaded, err := carv1.LoadCar(context.Background(), bs, f)
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

func writeRandomEDS(t *testing.T) *rsmt2d.ExtendedDataSquare {
	tmpDir := t.TempDir()
	err := os.Chdir(tmpDir)
	require.NoError(t, err, "error changing to the temporary test directory")
	f, err := os.OpenFile("test.car", os.O_WRONLY|os.O_CREATE, 0600)
	require.NoError(t, err, "error opening file")

	eds := share.RandEDS(t, 4)
	err = WriteEDS(context.Background(), eds, f)
	require.NoError(t, err, "error writing EDS to file")
	f.Close()
	return eds
}

func openWrittenEDS(t *testing.T) *os.File {
	f, err := os.OpenFile("test.car", os.O_RDONLY, 0600)
	require.NoError(t, err, "error opening file")
	return f
}
