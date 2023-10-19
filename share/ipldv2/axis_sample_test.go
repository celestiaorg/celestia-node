package ipldv2

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/share/eds/edstest"
)

func TestAxisSample(t *testing.T) {
	square := edstest.RandEDS(t, 2)

	aid, err := NewAxisSampleFromEDS(1, square, 2, rsmt2d.Row)
	require.NoError(t, err)

	data, err := aid.MarshalBinary()
	require.NoError(t, err)

	blk, err := aid.IPLDBlock()
	require.NoError(t, err)

	cid, err := aid.ID.Cid()
	require.NoError(t, err)
	assert.EqualValues(t, blk.Cid(), cid)

	sidOut := &AxisSample{}
	err = sidOut.UnmarshalBinary(data)
	require.NoError(t, err)
	assert.EqualValues(t, aid, sidOut)

	err = sidOut.Validate()
	require.NoError(t, err)
}
