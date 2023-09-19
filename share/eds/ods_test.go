package eds

import (
	"context"
	"io"
	"testing"

	"github.com/ipld/go-car"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/share"
)

// TestODSReader ensures that the reader returned from ODSReader is capable of reading the CAR
// header and ODS.
func TestODSReader(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	// launch eds store
	edsStore, err := newStore(t)
	require.NoError(t, err)
	err = edsStore.Start(ctx)
	require.NoError(t, err)

	// generate random eds data and put it into the store
	eds, dah := randomEDS(t)
	err = edsStore.Put(ctx, dah.Hash(), eds)
	require.NoError(t, err)

	// get CAR reader from store
	r, err := edsStore.GetCAR(ctx, dah.Hash())
	assert.NoError(t, err)
	defer func() {
		require.NoError(t, r.Close())
	}()

	// create ODSReader wrapper based on car reader to limit reads to ODS only
	odsR, err := ODSReader(r)
	assert.NoError(t, err)

	// create CAR reader from ODSReader
	carReader, err := car.NewCarReader(odsR)
	assert.NoError(t, err)

	// validate ODS could be obtained from reader
	for i := 0; i < 4; i++ {
		for j := 0; j < 4; j++ {
			// pick share from original eds
			original := eds.GetCell(uint(i), uint(j))

			// read block from odsReader based reader
			block, err := carReader.Next()
			assert.NoError(t, err)

			// check that original data from eds is same as data from reader
			assert.Equal(t, original, share.GetData(block.RawData()))
		}
	}

	// Make sure no excess data is available to get from reader
	_, err = carReader.Next()
	assert.Error(t, io.EOF, err)
}

// TestODSReaderReconstruction ensures that the reader returned from ODSReader provides sufficient
// data for EDS reconstruction
func TestODSReaderReconstruction(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	// launch eds store
	edsStore, err := newStore(t)
	require.NoError(t, err)
	err = edsStore.Start(ctx)
	require.NoError(t, err)

	// generate random eds data and put it into the store
	eds, dah := randomEDS(t)
	err = edsStore.Put(ctx, dah.Hash(), eds)
	require.NoError(t, err)

	// get CAR reader from store
	r, err := edsStore.GetCAR(ctx, dah.Hash())
	assert.NoError(t, err)
	defer func() {
		require.NoError(t, r.Close())
	}()

	// create ODSReader wrapper based on car reader to limit reads to ODS only
	odsR, err := ODSReader(r)
	assert.NoError(t, err)

	// reconstruct EDS from ODSReader
	loaded, err := ReadEDS(ctx, odsR, dah.Hash())
	assert.NoError(t, err)

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
