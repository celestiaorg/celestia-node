package shwap

import (
	"context"
	"fmt"
	"github.com/celestiaorg/celestia-node/share/store/file"
	"github.com/celestiaorg/rsmt2d"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/testing/edstest"
)

func TestSample(t *testing.T) {
	square := edstest.RandEDS(t, 8)
	root, err := share.NewRoot(square)
	require.NoError(t, err)

	sample, err := newSampleFromEDS(1, 1, square, 1)
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

// newSampleFromEDS samples the EDS and constructs a new row-proven Sample.
func newSampleFromEDS(
	x, y int,
	square *rsmt2d.ExtendedDataSquare,
	height uint64,
) (*Sample, error) {
	root, err := share.NewRoot(square)
	if err != nil {
		return nil, err
	}

	smplIdx := x + y*len(square.Row(0))
	id, err := NewSampleID(height, smplIdx, root)
	if err != nil {
		return nil, err
	}

	f := file.MemFile{Eds: square}
	shareWithProof, err := f.Share(context.TODO(), x, y)
	if err != nil {
		return nil, fmt.Errorf("while getting share: %w", err)
	}
	return &Sample{
		SampleID:       id,
		ShareWithProof: shareWithProof,
	}, nil
}
