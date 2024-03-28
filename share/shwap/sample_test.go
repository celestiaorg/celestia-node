package shwap

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds/edstest"
)

func TestSample(t *testing.T) {
	square := edstest.RandEDS(t, 8)
	root, err := share.NewRoot(square)
	require.NoError(t, err)

	sample, err := NewSampleFromEDS(RowProofType, 1, square, 1)
	require.NoError(t, err)

	data, err := sample.MarshalBinary()
	require.NoError(t, err)

	blk, err := sample.IPLDBlock()
	require.NoError(t, err)
	assert.EqualValues(t, blk.Cid(), sample.Cid())

	sampleOut := &Sample{}
	err = sampleOut.UnmarshalBinary(data)
	require.NoError(t, err)
	assert.EqualValues(t, sample, sampleOut)

	err = sampleOut.Verify(root)
	require.NoError(t, err)
}
