package shwap

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/share/eds/edstest"
)

func TestSample(t *testing.T) {
	square := edstest.RandEDS(t, 8)

	sample, err := NewSampleFromEDS(rsmt2d.Row, 2, square, 1)
	require.NoError(t, err)

	data, err := sample.MarshalBinary()
	require.NoError(t, err)

	blk, err := sample.IPLDBlock()
	require.NoError(t, err)

	cid, err := sample.SampleID.Cid()
	require.NoError(t, err)
	assert.EqualValues(t, blk.Cid(), cid)

	sampleOut := &Sample{}
	err = sampleOut.UnmarshalBinary(data)
	require.NoError(t, err)
	assert.EqualValues(t, sample, sampleOut)

	err = sampleOut.Validate()
	require.NoError(t, err)
}
