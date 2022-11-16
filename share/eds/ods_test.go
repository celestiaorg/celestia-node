package eds

import (
	"context"
	"fmt"
	"io"
	"testing"

	"github.com/ipld/go-car"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/share"
)

// TestODSReader ensures that the reader returned from DSReader is capable of reading the CAR header and ODS.
func TestODSReader(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	edsStore, err := newStore(t)
	require.NoError(t, err)
	err = edsStore.Start(ctx)
	require.NoError(t, err)

	eds, dah := randomEDS(t)
	err = edsStore.Put(ctx, dah, eds)
	require.NoError(t, err)

	r, err := edsStore.GetCAR(ctx, dah)
	assert.NoError(t, err)

	odsR := ODSReader(r)
	carReader, err := car.NewCarReader(odsR)
	assert.NoError(t, err)
	header := carReader.Header
	fmt.Println(header.Version)

	for i := 0; i < 4; i++ {
		for j := 0; j < 4; j++ {
			original := eds.GetCell(uint(i), uint(j))
			fmt.Println(i, j)
			block, err := carReader.Next()
			assert.NoError(t, err)
			assert.Equal(t, original, block.RawData()[share.NamespaceSize:])
		}
	}

	// no more data should be available from reader
	_, err = carReader.Next()
	assert.Error(t, io.EOF, err)
}
