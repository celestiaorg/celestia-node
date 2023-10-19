package ipldv2

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds/edstest"
)

func TestShareSampleID(t *testing.T) {
	square := edstest.RandEDS(t, 2)
	root, err := share.NewRoot(square)
	require.NoError(t, err)

	sid := NewShareSampleID(1, root, 2, rsmt2d.Row)

	id, err := sid.Cid()
	require.NoError(t, err)

	assert.EqualValues(t, shareSamplingCodec, id.Prefix().Codec)
	assert.EqualValues(t, shareSamplingMultihashCode, id.Prefix().MhType)
	assert.EqualValues(t, ShareSampleIDSize, id.Prefix().MhLength)

	data, err := sid.MarshalBinary()
	require.NoError(t, err)

	sidOut := ShareSampleID{}
	err = sidOut.UnmarshalBinary(data)
	require.NoError(t, err)
	assert.EqualValues(t, sid, sidOut)

	err = sidOut.Validate()
	require.NoError(t, err)
}
