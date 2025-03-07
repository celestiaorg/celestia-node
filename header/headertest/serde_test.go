package headertest

import (
	"testing"

	pubsub_pb "github.com/libp2p/go-libp2p-pubsub/pb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/blake2b"

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

func TestMsgIDEquivalency(t *testing.T) {
	randHeader := RandExtendedHeader(t)
	bin, err := randHeader.MarshalBinary()
	require.NoError(t, err)

	oldMsgIDFunc := func(message *pubsub_pb.Message) string {
		mID := func(data []byte) string {
			hash := blake2b.Sum256(data)
			return string(hash[:])
		}

		h, _ := header.UnmarshalExtendedHeader(message.Data)
		if h == nil || h.RawHeader.ValidateBasic() != nil {
			return mID(message.Data)
		}

		return h.Commit.BlockID.String()
	}

	inboundMsg := &pubsub_pb.Message{Data: bin}

	expectedHash := oldMsgIDFunc(inboundMsg)
	gotHash := header.MsgID(inboundMsg)

	assert.Equal(t, expectedHash, gotHash)
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
