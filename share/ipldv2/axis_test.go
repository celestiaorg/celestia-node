package ipldv2

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/share/eds/edstest"
)

func TestAxis(t *testing.T) {
	square := edstest.RandEDS(t, 2)

	aid, err := NewAxisFromEDS(rsmt2d.Row, 1, square, 2)
	require.NoError(t, err)

	data, err := aid.MarshalBinary()
	require.NoError(t, err)

	blk, err := aid.IPLDBlock()
	require.NoError(t, err)

	cid, err := aid.ID.Cid()
	require.NoError(t, err)
	assert.EqualValues(t, blk.Cid(), cid)

	sidOut := &Axis{}
	err = sidOut.UnmarshalBinary(data)
	require.NoError(t, err)
	assert.EqualValues(t, aid, sidOut)

	err = sidOut.Validate()
	require.NoError(t, err)
}
