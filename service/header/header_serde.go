package header

import (
	"github.com/tendermint/tendermint/pkg/da"
	core "github.com/tendermint/tendermint/types"

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

	return out, out.ValidateBasic()
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

// MarshalExtendedHeaderRequest serializes the given ExtendedHeaderRequest to bytes using protobuf.
// Paired with UnmarshalExtendedHeaderRequest.
func MarshalExtendedHeaderRequest(in *ExtendedHeaderRequest) ([]byte, error) {
	out := &header_pb.ExtendedHeaderRequest{
		Origin: in.Origin,
		Amount: in.Amount,
	}
	return out.Marshal()
}

// UnmarshalExtendedHeaderRequest deserializes given data into a new ExtendedHeader using protobuf.
// Paired with MarshalExtendedHeaderRequest.
func UnmarshalExtendedHeaderRequest(data []byte) (*ExtendedHeaderRequest, error) {
	in := &header_pb.ExtendedHeaderRequest{}
	err := in.Unmarshal(data)
	if err != nil {
		return nil, err
	}
	return &ExtendedHeaderRequest{
		Origin: in.Origin,
		Amount: in.Amount,
	}, nil
}
