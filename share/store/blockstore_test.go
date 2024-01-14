package store

import (
	"context"
	"github.com/celestiaorg/celestia-node/share/shwap"
	"testing"

	boxobs "github.com/ipfs/boxo/blockstore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds"
	"github.com/celestiaorg/celestia-node/share/eds/edstest"
)

// TODO(@Wondertan): Add row and data code

func TestBlockstoreGetShareSample(t *testing.T) {
	ctx := context.Background()
	sqr := edstest.RandEDS(t, 4)
	root, err := share.NewRoot(sqr)
	require.NoError(t, err)

	b := edsBlockstore(sqr)

	width := int(sqr.Width())
	for i := 0; i < width*width; i++ {
		id, err := shwap.NewSampleID(1, i, root)
		require.NoError(t, err)

		blk, err := b.Get(ctx, id.Cid())
		require.NoError(t, err)

		sample, err := shwap.SampleFromBlock(blk)
		require.NoError(t, err)

		err = sample.Verify(root)
		require.NoError(t, err)
		assert.EqualValues(t, id, sample.SampleID)
	}
}

type edsFileAndFS eds.MemFile

func (m *edsFileAndFS) File(uint64) (*eds.MemFile, error) {
	return (*eds.MemFile)(m), nil
}

func edsBlockstore(sqr *rsmt2d.ExtendedDataSquare) boxobs.Blockstore {
	return NewBlockstore[*eds.MemFile]((*edsFileAndFS)(&eds.MemFile{Eds: sqr}))
}
