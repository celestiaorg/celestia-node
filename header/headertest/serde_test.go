package headertest

import (
	"testing"

	pubsub_pb "github.com/libp2p/go-libp2p-pubsub/pb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/blake2b"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/share/eds/edstest"
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
		if h == nil || h.ValidateBasic() != nil {
			return mID(message.Data)
		}

		return h.Commit.BlockID.String()
	}

	inboundMsg := &pubsub_pb.Message{Data: bin}

	expectedHash := oldMsgIDFunc(inboundMsg)
	gotHash := header.MsgID(inboundMsg)

	assert.Equal(t, expectedHash, gotHash)
}

// Before changes (with 256 EDS and 100 validators):
// BenchmarkMsgID-8   	    5203	    224681 ns/op	  511253 B/op	    4252 allocs/op
// After changes (with 256 EDS and 100 validators):
// BenchmarkMsgID-8   	   23559	     48399 ns/op	  226858 B/op	    1282 allocs/op
func BenchmarkMsgID(b *testing.B) {
	eds := edstest.RandomAxisRoots(b, 256)
	randHeader := RandExtendedHeader(b, WithDAH(eds))
	bin, err := randHeader.MarshalBinary()
	require.NoError(b, err)
	msg := &pubsub_pb.Message{Data: bin}

	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = header.MsgID(msg)
	}
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
