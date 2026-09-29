package header

import (
	core "github.com/cometbft/cometbft/types"
	pb "github.com/libp2p/go-libp2p-pubsub/pb"
	"golang.org/x/crypto/blake2b"

	"github.com/celestiaorg/celestia-app/v10/pkg/da"

	header_pb "github.com/celestiaorg/celestia-node/header/pb"
)

// MarshalExtendedHeader serializes given ExtendedHeader to bytes using protobuf.
// Paired with UnmarshalExtendedHeader.
func MarshalExtendedHeader(in *ExtendedHeader) (_ []byte, err error) {
	out := &header_pb.ExtendedHeader{
		Header: in.ToProto(),
		Commit: in.Commit.ToProto(),
	}

	out.ValidatorSet, err = in.ValidatorSet.ToProto()
	if err != nil {
		return nil, err
	}

	out.Dah, err = in.DAH.ToProto()
	if err != nil {
		return nil, err
	}

	return out.Marshal()
}

// UnmarshalExtendedHeader deserializes given data into a new ExtendedHeader using protobuf.
// Paired with MarshalExtendedHeader.
func UnmarshalExtendedHeader(data []byte) (*ExtendedHeader, error) {
	in := &header_pb.ExtendedHeader{}
	err := in.Unmarshal(data)
	if err != nil {
		return nil, err
	}

	out := &ExtendedHeader{}
	out.RawHeader, err = core.HeaderFromProto(in.Header)
	if err != nil {
		return nil, err
	}

	out.Commit, err = core.CommitFromProto(in.Commit)
	if err != nil {
		return nil, err
	}

	out.ValidatorSet, err = core.ValidatorSetFromProto(in.ValidatorSet)
	if err != nil {
		return nil, err
	}

	out.DAH, err = da.DataAvailabilityHeaderFromProto(in.Dah)
	if err != nil {
		return nil, err
	}

	return out, nil
}

// MsgID computes an id for a pubsub message.
//
// The id is a hash over the raw message bytes, so it can't be declared by the
// sender and doesn't require decoding untrusted data before validation.
//
// NOTE: Validators don't necessarily collect commit signatures from the entire
// validator set, so Bridge Nodes connected to different validators may gossip
// the same header with different commit signature sets, and thus different ids.
// Such duplicates are ignored by the header-sub validator as known headers.
func MsgID(pmsg *pb.Message) string {
	hash := blake2b.Sum256(pmsg.GetData())
	return string(hash[:])
}
