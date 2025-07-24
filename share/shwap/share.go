package shwap

import (
	"fmt"

	libshare "github.com/celestiaorg/go-square/v2/share"

	"github.com/celestiaorg/celestia-node/share/shwap/pb"
)

// ShareFromProto converts a protobuf Share object to the application's internal share
// representation. It returns nil if the input protobuf Share is nil, ensuring safe handling of nil
// values.
func ShareFromProto(s *pb.Share) (libshare.Share, error) {
	if s == nil {
		return libshare.Share{}, nil
	}
	sh, err := libshare.NewShare(s.Data)
	if err != nil {
		return libshare.Share{}, err
	}
	return *sh, nil
}

// SharesToProto converts a slice of Shares from the application's internal representation to a
// slice of protobuf Share objects. This function allocates memory for the protobuf objects and
// copies data from the input slice.
func SharesToProto(shrs []libshare.Share) []*pb.Share {
	protoShares := make([]*pb.Share, len(shrs))
	for i, shr := range shrs {
		protoShares[i] = &pb.Share{Data: shr.ToBytes()}
	}
	return protoShares
}

// SharesFromProto converts a slice of protobuf Share objects to the application's internal slice
// of Shares. It ensures that each Share is correctly transformed using the ShareFromProto function.
func SharesFromProto(shrs []*pb.Share) ([]libshare.Share, error) {
	shares := make([]libshare.Share, len(shrs))
	var err error
	for i, shr := range shrs {
		shares[i], err = ShareFromProto(shr)
		if err != nil {
			return nil, fmt.Errorf("invalid share at index %d: %w", i, err)
		}
	}
	return shares, nil
}
