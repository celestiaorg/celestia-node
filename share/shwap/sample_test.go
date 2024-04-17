package shwap

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/testing/edstest"
)

func TestSample(t *testing.T) {
	square := edstest.RandEDS(t, 8)
	root, err := share.NewRoot(square)
	require.NoError(t, err)

	sample, err := newSampleFromEDS(square, 1, rsmt2d.Row, 0, 1)
	require.NoError(t, err)

	// test block encoding
	blk, err := sample.IPLDBlock()
	require.NoError(t, err)
	assert.EqualValues(t, blk.Cid(), sample.Cid())

	sampleOut, err := SampleFromBlock(blk)
	require.NoError(t, err)
	assert.EqualValues(t, sample, sampleOut)

	// test proto encoding
	pb := sample.ToProto()
	sampleOut, err = SampleFromProto(pb)
	require.NoError(t, err)
	assert.EqualValues(t, sample, sampleOut)

	err = sampleOut.Verify(root)
	require.NoError(t, err)
}

func newSampleFromEDS(
	square *rsmt2d.ExtendedDataSquare,
	height uint64,
	proofAxis rsmt2d.Axis,
	axisIdx, shrIdx int,
) (*Sample, error) {
	smplIdx := uint(shrIdx) + uint(axisIdx)*(square.Width())
	if proofAxis == rsmt2d.Col {
		smplIdx = uint(axisIdx) + uint(shrIdx)*(square.Width())
	}
	root, err := share.NewRoot(square)
	if err != nil {
		return nil, err
	}

	id, err := NewSampleID(height, int(smplIdx), root)
	if err != nil {
		return nil, err
	}
	sp, err := share.ShareWithProofFromEDS(square, proofAxis, int(axisIdx), int(shrIdx))
	if err != nil {
		return nil, err
	}
	return &Sample{SampleID: id, ShareWithProof: sp}, nil
}
