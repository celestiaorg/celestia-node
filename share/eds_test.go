package share

import (
	"context"
	"github.com/celestiaorg/celestia-node/ipld"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestQuadrantOrder(t *testing.T) {
	//result, _ := rsmt2d.ComputeExtendedDataSquare([][]byte{
	//	{1}, {2},
	//	{3}, {4},
	//}, rsmt2d.NewRSGF8Codec(), rsmt2d.NewDefaultTree)
	// TODO: make this into an actual test
	//fmt.Println(quadrantOrder(result))
}

func TestWriteEDS(t *testing.T) {
	tmpDir := t.TempDir()
	err := os.Chdir(tmpDir)
	require.NoError(t, err, "error changing to the temporary test directory")
	f, err := os.OpenFile("test.car", os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0600)
	require.NoError(t, err, "error opening file")
	defer f.Close()

	eds := ipld.RandEDS(t, 4)
	err = WriteEDS(context.Background(), eds, f)
	require.NoError(t, err, "error writing EDS to file")
}
