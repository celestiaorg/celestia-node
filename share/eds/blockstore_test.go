package eds

import (
	"context"
	"io"
	"testing"

	"github.com/filecoin-project/dagstore"
	"github.com/ipld/go-car"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBlockstore_Operations tests Has, Get, and GetSize on the top level eds.Store blockstore.
// It verifies that these operations are valid and successful on all blocks stored in a CAR file.
func TestBlockstore_Operations(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	edsStore, err := newStore(t)
	require.NoError(t, err)
	err = edsStore.Start(ctx)
	require.NoError(t, err)

	eds, dah := randomEDS(t)
	err = edsStore.Put(ctx, dah.Hash(), eds)
	require.NoError(t, err)

	r, err := edsStore.GetCAR(ctx, dah.Hash())
	require.NoError(t, err)
	carReader, err := car.NewCarReader(r)
	require.NoError(t, err)

	topLevelBS := edsStore.Blockstore()
	carBS, _, err := edsStore.CARBlockstore(ctx, dah.Hash())
	require.NoError(t, err)

	blockstores := []dagstore.ReadBlockstore{topLevelBS, carBS}

	for {
		next, err := carReader.Next()
		if err != nil {
			require.ErrorIs(t, err, io.EOF)
			break
		}
		blockCid := next.Cid()

		for _, bs := range blockstores {
			// test GetSize
			has, err := bs.Has(ctx, blockCid)
			require.NoError(t, err, "blockstore.Has could not find root CID")
			require.True(t, has)

			// test GetSize
			block, err := bs.Get(ctx, blockCid)
			assert.NoError(t, err, "blockstore.Get could not get a leaf CID")
			assert.Equal(t, block.Cid(), blockCid)
			assert.Equal(t, block.RawData(), next.RawData())

			// test GetSize
			size, err := bs.GetSize(ctx, blockCid)
			assert.NotZerof(t, size, "blocksize.GetSize reported a root block from blockstore was empty")
			assert.NoError(t, err)
		}
	}
}
