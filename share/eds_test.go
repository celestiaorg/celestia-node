package share

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/ipld"
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
	f, err := os.OpenFile("/tmp/123.car", os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0600)
	if err != nil {
		panic(err)
	}
	defer f.Close()

	eds := ipld.RandEDS(t, 2)
	err = WriteEDS(context.Background(), eds, f)
	require.Nil(t, err)
}
