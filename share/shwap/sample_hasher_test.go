package shwap

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/share/testing/edstest"
)

func TestSampleHasher(t *testing.T) {
	hasher := &SampleHasher{}

	_, err := hasher.Write([]byte("hello"))
	assert.Error(t, err)

	square := edstest.RandEDS(t, 2)
	sample, err := newSampleFromEDS(square, 1, rsmt2d.Row, 1, 1)
	require.NoError(t, err)

	data, err := sample.ToProto().Marshal()
	require.NoError(t, err)

	n, err := hasher.Write(data)
	require.NoError(t, err)
	assert.EqualValues(t, len(data), n)

	digest := hasher.Sum(nil)
	id := sample.SampleID.MarshalBinary()
	assert.EqualValues(t, id, digest)

	hasher.Reset()
	digest = hasher.Sum(nil)
	assert.NotEqualValues(t, digest, id)
}
