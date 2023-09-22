package ipldv2

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/nmt"
	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds"
	"github.com/celestiaorg/celestia-node/share/eds/edstest"
)

func TestBlockstoreGet(t *testing.T) {
	ctx := context.Background()
	sqr := edstest.RandEDS(t, 4)
	root, err := share.NewRoot(sqr)
	require.NoError(t, err)

	path := t.TempDir() + "/eds_file"
	f, err := eds.CreateFile(path, sqr)
	require.NoError(t, err)
	b := NewBlockstore[*edsFileAndFS]((*edsFileAndFS)(f))

	axis := []rsmt2d.Axis{rsmt2d.Row, rsmt2d.Col}
	width := int(sqr.Width())
	for _, axis := range axis {
		for i := 0; i < width*width; i++ {
			id := NewSampleID(root, i, axis)
			cid, err := id.Cid()
			require.NoError(t, err)

			blk, err := b.Get(ctx, cid)
			require.NoError(t, err)

			sample, err := SampleFromBlock(blk)
			require.NoError(t, err)

			err = sample.Validate()
			require.NoError(t, err)
			assert.EqualValues(t, id, sample.ID)
		}
	}
}

type edsFileAndFS eds.File

func (m *edsFileAndFS) File(share.DataHash) (*edsFileAndFS, error) {
	return m, nil
}

func (m *edsFileAndFS) Header() *eds.Header {
	return (*eds.File)(m).Header()
}

func (m *edsFileAndFS) ShareWithProof(idx int, axis rsmt2d.Axis) (share.Share, nmt.Proof, error) {
	return (*eds.File)(m).ShareWithProof(idx, axis)
}

func (m *edsFileAndFS) Close() error {
	return nil
}
