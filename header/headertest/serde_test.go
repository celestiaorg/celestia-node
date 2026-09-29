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

// TestMsgID ensures the msg id binds the whole message rather than the
// sender-declared Commit.BlockID.
func TestMsgID(t *testing.T) {
	randHeader := RandExtendedHeader(t)
	bin, err := randHeader.MarshalBinary()
	require.NoError(t, err)

	hash := blake2b.Sum256(bin)
	assert.Equal(t, string(hash[:]), header.MsgID(&pubsub_pb.Message{Data: bin}))

	// a message declaring the same BlockID but carrying a different body
	// must not share the id of the genuine message
	forged := *randHeader
	commit := *randHeader.Commit
	commit.Signatures = nil
	forged.Commit = &commit
	forgedBin, err := forged.MarshalBinary()
	require.NoError(t, err)
	require.Equal(t, randHeader.Commit.BlockID, forged.Commit.BlockID)
	assert.NotEqual(t,
		header.MsgID(&pubsub_pb.Message{Data: bin}),
		header.MsgID(&pubsub_pb.Message{Data: forgedBin}),
	)
}

func BenchmarkMsgID(b *testing.B) {
	eds := edstest.RandomAxisRoots(b, 256)
	randHeader := RandExtendedHeaderWithRoot(b, eds)
	bin, err := randHeader.MarshalBinary()
	require.NoError(b, err)
	msg := &pubsub_pb.Message{Data: bin}

	b.ReportAllocs()

	for b.Loop() {
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
