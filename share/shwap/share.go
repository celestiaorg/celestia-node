package shwap

import (
	"fmt"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/shwap/pb"
)

// ShareFromProto converts a protobuf Share object to the application's internal share
// representation. It returns nil if the input protobuf Share is nil, ensuring safe handling of nil
// values.
func ShareFromProto(s *pb.Share) share.Share {
	if s == nil {
		return nil
	}
	return s.Data
}

// SharesToProto converts a slice of Shares from the application's internal representation to a
// slice of protobuf Share objects. This function allocates memory for the protobuf objects and
// copies data from the input slice.
func SharesToProto(shrs []share.Share) []*pb.Share {
	protoShares := make([]*pb.Share, len(shrs))
	for i, shr := range shrs {
		protoShares[i] = &pb.Share{Data: shr}
	}
	return protoShares
}

// SharesFromProto converts a slice of protobuf Share objects to the application's internal slice
// of Shares. It ensures that each Share is correctly transformed using the ShareFromProto function.
func SharesFromProto(shrs []*pb.Share) []share.Share {
	shares := make([]share.Share, len(shrs))
	for i, shr := range shrs {
		shares[i] = ShareFromProto(shr)
	}
	return shares
}

func ValidateShares(shares []share.Share) error {
	for i, shr := range shares {
		if err := share.ValidateShare(shr); err != nil {
			return fmt.Errorf("while validating share at index %d: %w", i, err)
		}
	}
	return nil
}
