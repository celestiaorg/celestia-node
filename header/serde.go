package header

import (
	pb "github.com/libp2p/go-libp2p-pubsub/pb"
	core "github.com/tendermint/tendermint/types"
	"golang.org/x/crypto/blake2b"

	"github.com/celestiaorg/celestia-app/pkg/da"

	header_pb "github.com/celestiaorg/celestia-node/header/pb"
)

// MarshalExtendedHeader serializes given ExtendedHeader to bytes using protobuf.
// Paired with UnmarshalExtendedHeader.
func MarshalExtendedHeader(in *ExtendedHeader) (_ []byte, err error) {
	out := &header_pb.ExtendedHeader{
		Header: in.RawHeader.ToProto(),
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

func ExtendedHeaderToProto(eh *ExtendedHeader) (*header_pb.ExtendedHeader, error) {
	pb := &header_pb.ExtendedHeader{
		Header: eh.RawHeader.ToProto(),
		Commit: eh.Commit.ToProto(),
	}
	valSet, err := eh.ValidatorSet.ToProto()
	if err != nil {
		return nil, err
	}
	pb.ValidatorSet = valSet
	dah, err := eh.DAH.ToProto()
	if err != nil {
		return nil, err
	}
	pb.Dah = dah
	return pb, nil
}

func ProtoToExtendedHeader(pb *header_pb.ExtendedHeader) (*ExtendedHeader, error) {
	bin, err := pb.Marshal()
	if err != nil {
		return nil, err
	}
	header := new(ExtendedHeader)
	err = header.UnmarshalBinary(bin)
	if err != nil {
		return nil, err
	}
	return header, nil
}

// msgID computes an id for a pubsub message
// TODO(@Wondertan): This cause additional allocations per each recvd message in the topic
//
//	Find a way to avoid those.
func MsgID(pmsg *pb.Message) string {
	mID := func(data []byte) string {
		hash := blake2b.Sum256(data)
		return string(hash[:])
	}

	h, _ := UnmarshalExtendedHeader(pmsg.Data)
	if h == nil || h.RawHeader.ValidateBasic() != nil {
		// There is nothing we can do about the error, and it will be anyway caught during validation.
		// We also *have* to return some ID for the msg, so give the hash of even faulty msg
		return mID(pmsg.Data)
	}

	// IMPORTANT NOTE:
	// Due to the nature of the Tendermint consensus, validators don't necessarily collect commit
	// signatures from the entire validator set, but only the minimum required amount of them (>2/3 of
	// voting power). In addition, signatures are collected asynchronously. Therefore, each validator
	// may have a different set of signatures that pass the minimum required voting power threshold,
	// causing nondeterminism in the header message gossiped over the network. Subsequently, this
	// causes message duplicates as each Bridge Node, connected to a personal validator, sends the
	// validator's own view of commits of effectively the same header.
	//
	// To solve the nondeterminism problem above, we don't compute msg id on message body and take
	// the actual header hash as an id.
	return h.Commit.BlockID.String()
}
