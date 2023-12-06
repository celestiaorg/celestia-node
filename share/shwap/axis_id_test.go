package shwap

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds/edstest"
)

func TestAxisID(t *testing.T) {
	square := edstest.RandEDS(t, 2)
	root, err := share.NewRoot(square)
	require.NoError(t, err)

	sid := NewAxisID(rsmt2d.Row, 2, root, 1)

	id, err := sid.Cid()
	require.NoError(t, err)

	assert.EqualValues(t, axisCodec, id.Prefix().Codec)
	assert.EqualValues(t, axisMultihashCode, id.Prefix().MhType)
	assert.EqualValues(t, AxisIDSize, id.Prefix().MhLength)

	data, err := sid.MarshalBinary()
	require.NoError(t, err)

	sidOut := AxisID{}
	err = sidOut.UnmarshalBinary(data)
	require.NoError(t, err)
	assert.EqualValues(t, sid, sidOut)

	err = sidOut.Validate()
	require.NoError(t, err)
}
