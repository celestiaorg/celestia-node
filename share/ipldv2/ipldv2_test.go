package ipldv2

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/rsmt2d"

	availability_test "github.com/celestiaorg/celestia-node/share/availability/test"
	"github.com/celestiaorg/celestia-node/share/eds/edstest"
)

// TestV2Roundtrip tests full protocol round trip of:
// EDS -> Sample -> IPLDBlock -> BlockService -> Bitswap and in reverse.
func TestV2Roundtrip(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	dn := availability_test.NewTestDAGNet(ctx, t)
	srv1 := dn.NewTestNode().BlockService
	srv2 := dn.NewTestNode().BlockService
	dn.ConnectAll()

	square := edstest.RandEDS(t, 16)
	width := int(square.Width())
	for i := 0; i < width*width; i++ {
		smpl, err := NewSampleFrom(square, i, rsmt2d.Row)
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
