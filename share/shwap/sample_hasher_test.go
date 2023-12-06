package shwap

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/share/eds/edstest"
)

func TestSampleHasher(t *testing.T) {
	hasher := &SampleHasher{}

	_, err := hasher.Write([]byte("hello"))
	assert.Error(t, err)

	square := edstest.RandEDS(t, 2)

	sample, err := NewSampleFromEDS(rsmt2d.Row, 2, square, 1)
	require.NoError(t, err)

	data, err := sample.MarshalBinary()
	require.NoError(t, err)

	n, err := hasher.Write(data)
	require.NoError(t, err)
	assert.EqualValues(t, len(data), n)

	digest := hasher.Sum(nil)
	sid, err := sample.SampleID.MarshalBinary()
	require.NoError(t, err)
	assert.EqualValues(t, sid, digest)

	hasher.Reset()
	digest = hasher.Sum(nil)
	assert.NotEqualValues(t, digest, sid)
}
