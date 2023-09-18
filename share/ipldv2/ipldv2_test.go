package ipldv2

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/share"
	availability_test "github.com/celestiaorg/celestia-node/share/availability/test"
	"github.com/celestiaorg/celestia-node/share/eds"
	"github.com/celestiaorg/celestia-node/share/eds/edstest"
)

func TestV2Roundtrip(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	dn := availability_test.NewTestDAGNet(ctx, t)
	srv1 := dn.NewTestNode().BlockService
	srv2 := dn.NewTestNode().BlockService
	dn.ConnectAll()

	square := edstest.RandEDS(t, 16)
	root, err := share.NewRoot(square)
	require.NoError(t, err)

	file, err := eds.CreateFile(t.TempDir()+"/eds_file", square)
	require.NoError(t, err)

	width := int(square.Width())
	for i := 0; i < width*width; i++ {
		shr, prf, err := file.ShareWithProof(i, rsmt2d.Row)
		require.NoError(t, err)

		smpl := NewSample(root, i, rsmt2d.Row, shr, prf)
		require.NoError(t, err)

		err = smpl.Validate()
		require.NoError(t, err)

		blkIn, err := smpl.IPLDBlock()
		require.NoError(t, err)

		err = srv1.AddBlock(ctx, blkIn)
		require.NoError(t, err)

		blkOut, err := srv2.GetBlock(ctx, blkIn.Cid())
		require.NoError(t, err)

		assert.EqualValues(t, blkIn.RawData(), blkOut.RawData())
		assert.EqualValues(t, blkIn.Cid(), blkOut.Cid())
	}
}
