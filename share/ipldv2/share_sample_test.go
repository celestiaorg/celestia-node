package ipldv2

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/share/eds/edstest"
)

func TestShareSample(t *testing.T) {
	square := edstest.RandEDS(t, 2)

	sid, err := NewShareSampleFromEDS(square, 2, rsmt2d.Row)
	require.NoError(t, err)

	data, err := sid.MarshalBinary()
	require.NoError(t, err)

	blk, err := sid.IPLDBlock()
	require.NoError(t, err)

	cid, err := sid.ID.Cid()
	require.NoError(t, err)
	assert.EqualValues(t, blk.Cid(), cid)

	sidOut := &ShareSample{}
	err = sidOut.UnmarshalBinary(data)
	require.NoError(t, err)
	assert.EqualValues(t, sid, sidOut)

	err = sidOut.Validate()
	require.NoError(t, err)
}
