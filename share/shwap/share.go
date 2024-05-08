package shwap

import (
	"github.com/celestiaorg/celestia-node/share"
	types_pb "github.com/celestiaorg/celestia-node/share/shwap/proto"
)

// ShareFromProto converts a protobuf Share object to the application's internal share
// representation. It returns nil if the input protobuf Share is nil, ensuring safe handling of nil
// values.
func ShareFromProto(s *types_pb.Share) share.Share {
	if s == nil {
		return nil
	}
	return s.Data
}

// SharesToProto converts a slice of Shares from the application's internal representation to a
// slice of protobuf Share objects. This function allocates memory for the protobuf objects and
// copies data from the input slice.
func SharesToProto(shrs []share.Share) []*types_pb.Share {
	protoShares := make([]*types_pb.Share, len(shrs))
	for i, shr := range shrs {
		protoShares[i] = &types_pb.Share{Data: shr}
	}
	return protoShares
}

// SharesFromProto converts a slice of protobuf Share objects to the application's internal slice
// of Shares. It ensures that each Share is correctly transformed using the ShareFromProto function.
func SharesFromProto(shrs []*types_pb.Share) []share.Share {
	shares := make([]share.Share, len(shrs))
	for i, shr := range shrs {
		shares[i] = ShareFromProto(shr)
	}
	return shares
}
