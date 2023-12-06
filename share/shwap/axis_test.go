package shwap

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/share/eds/edstest"
)

func TestAxis(t *testing.T) {
	square := edstest.RandEDS(t, 8)

	axis, err := NewAxisFromEDS(rsmt2d.Row, 1, square, 2)
	require.NoError(t, err)

	data, err := axis.MarshalBinary()
	require.NoError(t, err)

	blk, err := axis.IPLDBlock()
	require.NoError(t, err)

	cid, err := axis.AxisID.Cid()
	require.NoError(t, err)
	assert.EqualValues(t, blk.Cid(), cid)

	axisOut := &Axis{}
	err = axisOut.UnmarshalBinary(data)
	require.NoError(t, err)
	assert.EqualValues(t, axis, axisOut)

	err = axisOut.Validate()
	require.NoError(t, err)
}
