package header

import (
	"github.com/celestiaorg/celestia-core/pkg/da"
	core "github.com/celestiaorg/celestia-core/types"

	header_pb "github.com/celestiaorg/celestia-node/service/header/pb"
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

// MarshalExtendedHeader deserializes given data into a new ExtendedHeader using protobuf.
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
