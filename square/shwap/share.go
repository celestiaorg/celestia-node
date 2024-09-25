package shwap

import (
	"fmt"

	"github.com/celestiaorg/go-square/v2/share"

	"github.com/celestiaorg/celestia-node/square/shwap/pb"
)

// ShareFromProto converts a protobuf Share object to the application's internal share
// representation. It returns nil if the input protobuf Share is nil, ensuring safe handling of nil
// values.
func ShareFromProto(s *pb.Share) (share.Share, error) {
	if s == nil {
		return share.Share{}, nil
	}
	sh, err := share.NewShare(s.Data)
	if err != nil {
		return share.Share{}, err
	}
	return *sh, err
}

// SharesToProto converts a slice of Shares from the application's internal representation to a
// slice of protobuf Share objects. This function allocates memory for the protobuf objects and
// copies data from the input slice.
func SharesToProto(shrs []share.Share) []*pb.Share {
	protoShares := make([]*pb.Share, len(shrs))
	for i, shr := range shrs {
		protoShares[i] = &pb.Share{Data: shr.ToBytes()}
	}
	return protoShares
}

// SharesFromProto converts a slice of protobuf Share objects to the application's internal slice
// of Shares. It ensures that each Share is correctly transformed using the ShareFromProto function.
func SharesFromProto(shrs []*pb.Share) ([]share.Share, error) {
	shares := make([]share.Share, len(shrs))
	var err error
	for i, shr := range shrs {
		shares[i], err = ShareFromProto(shr)
		if err != nil {
			return nil, fmt.Errorf("invalid share at index %d: %w", i, err)
		}
	}
	return shares, nil
}
