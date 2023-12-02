package ipldv2

import (
	"context"
	"testing"

	"github.com/ipfs/boxo/blockstore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds"
	"github.com/celestiaorg/celestia-node/share/eds/edstest"
)

// TODO(@Wondertan): Add axis and data code

func TestBlockstoreGetShareSample(t *testing.T) {
	ctx := context.Background()
	sqr := edstest.RandEDS(t, 4)
	root, err := share.NewRoot(sqr)
	require.NoError(t, err)

	b := edsBlockstore(sqr)

	width := int(sqr.Width())
	for _, axisType := range axisTypes {
		for i := 0; i < width*width; i++ {
			id := NewSampleID(axisType, i, root, 1)
			cid, err := id.Cid()
			require.NoError(t, err)

			blk, err := b.Get(ctx, cid)
			require.NoError(t, err)

			sample, err := SampleFromBlock(blk)
			require.NoError(t, err)

			err = sample.Validate()
			require.NoError(t, err)
			assert.EqualValues(t, id, sample.SampleID)
		}
	}
}

type edsFileAndFS eds.MemFile

func (m *edsFileAndFS) File(uint64) (*eds.MemFile, error) {
	return (*eds.MemFile)(m), nil
}

func edsBlockstore(sqr *rsmt2d.ExtendedDataSquare) blockstore.Blockstore {
	return NewBlockstore[*eds.MemFile]((*edsFileAndFS)(&eds.MemFile{Eds: sqr}))
}
