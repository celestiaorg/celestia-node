package share

import (
	"github.com/celestiaorg/celestia-app/pkg/appconsts"
	types_pb "github.com/celestiaorg/celestia-node/share/pb"
)

var (
	// DefaultRSMT2DCodec sets the default rsmt2d.Codec for shares.
	DefaultRSMT2DCodec = appconsts.DefaultCodec
)

const (
	// Size is a system-wide size of a share, including both data and namespace GetNamespace
	Size = appconsts.ShareSize
)

var (
	// MaxSquareSize is currently the maximum size supported for unerasured data in
	// rsmt2d.ExtendedDataSquare.
	MaxSquareSize = appconsts.SquareSizeUpperBound(appconsts.LatestVersion)
)

// Share contains the raw share data without the corresponding namespace.
// NOTE: Alias for the byte is chosen to keep maximal compatibility, especially with rsmt2d.
// Ideally, we should define reusable type elsewhere and make everyone(Core, rsmt2d, ipld) to rely
// on it.
type Share = []byte

// GetNamespace slices Namespace out of the Share.
func GetNamespace(s Share) Namespace {
	return s[:NamespaceSize]
}

// GetData slices out data of the Share.
func GetData(s Share) []byte {
	return s[NamespaceSize:]
}

func ShareFromProto(s *types_pb.Share) Share {
	if s == nil {
		return nil
	}
	return s.Data
}
