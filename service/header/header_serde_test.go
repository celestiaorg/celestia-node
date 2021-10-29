package header

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMarshalUnmarshalExtendedHeader(t *testing.T) {
	in := RandExtendedHeader(t)
	data, err := in.MarshalBinary()
	require.NoError(t, err)

	out := &ExtendedHeader{}
	err = out.UnmarshalBinary(data)
	require.NoError(t, err)
	assert.Equal(t, in.ValidatorSet, out.ValidatorSet)
	assert.True(t, in.DAH.Equals(out.DAH))
	// not the check for equality as time.Time is not serialized exactly 1:1
	assert.NotZero(t, out.RawHeader)
	assert.NotNil(t, out.Commit)
}
