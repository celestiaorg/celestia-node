package headertest

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/header"
)

func TestMarshalUnmarshalExtendedHeader(t *testing.T) {
	in := RandExtendedHeader(t)
	binaryData, err := in.MarshalBinary()
	require.NoError(t, err)

	out := &header.ExtendedHeader{}
	err = out.UnmarshalBinary(binaryData)
	require.NoError(t, err)
	equalExtendedHeader(t, in, out)

	// A custom JSON marshal/unmarshal is necessary which wraps the ValidatorSet with amino
	// encoding, to be able to marshal the crypto.PubKey type back from JSON.
	jsonData, err := in.MarshalJSON()
	require.NoError(t, err)

	out = &header.ExtendedHeader{}
	err = out.UnmarshalJSON(jsonData)
	require.NoError(t, err)
	equalExtendedHeader(t, in, out)
}

func equalExtendedHeader(t *testing.T, in, out *header.ExtendedHeader) {
	// ValidatorSet.totalVotingPower is not set (is a cached value that can be recomputed client side)
	assert.Equal(t, in.ValidatorSet.Validators, out.ValidatorSet.Validators)
	assert.Equal(t, in.ValidatorSet.Proposer, out.ValidatorSet.Proposer)
	assert.True(t, in.DAH.Equals(out.DAH))
	// not the check for equality as time.Time is not serialized exactly 1:1
	assert.NotZero(t, out.RawHeader)
	assert.NotNil(t, out.Commit)
}
