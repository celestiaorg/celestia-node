package shwap

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/testing/edstest"
)

func TestSampleID(t *testing.T) {
	square := edstest.RandEDS(t, 2)
	root, err := share.NewRoot(square)
	require.NoError(t, err)

	id, err := NewSampleID(1, 1, root)
	require.NoError(t, err)

	cid := id.Cid()
	assert.EqualValues(t, sampleCodec, cid.Prefix().Codec)
	assert.EqualValues(t, sampleMultihashCode, cid.Prefix().MhType)
	assert.EqualValues(t, SampleIDSize, cid.Prefix().MhLength)

	data := id.MarshalBinary()
	idOut, err := SampleIdFromBinary(data)
	require.NoError(t, err)
	assert.EqualValues(t, id, idOut)

	err = idOut.Verify(root)
	require.NoError(t, err)
}
