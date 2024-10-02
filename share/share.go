package share

import (
	"crypto/sha256"
	"fmt"

	"github.com/celestiaorg/celestia-app/v2/pkg/appconsts"
	"github.com/celestiaorg/go-square/shares"
	"github.com/celestiaorg/nmt"
	"github.com/celestiaorg/rsmt2d"
)

// DefaultRSMT2DCodec sets the default rsmt2d.Codec for shares.
var DefaultRSMT2DCodec = appconsts.DefaultCodec

const (
	// Size is a system-wide size of a share, including both data and namespace GetNamespace
	Size = appconsts.ShareSize
)

// MaxSquareSize is currently the maximum size supported for unerasured data in
// rsmt2d.ExtendedDataSquare.
var MaxSquareSize = appconsts.SquareSizeUpperBound(appconsts.LatestVersion)

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

// ValidateShare checks the size of a given share.
func ValidateShare(s Share) error {
	if len(s) != Size {
		return fmt.Errorf("invalid share size: %d", len(s))
	}
	return nil
}

// TailPadding is a constant tail padding share exported for reuse
func TailPadding() Share {
	return tailPadding
}

// ShareWithProof contains data with corresponding Merkle Proof
type ShareWithProof struct { //nolint: revive
	// Share is a full data including namespace
	Share
	// Proof is a Merkle Proof of current share
	Proof *nmt.Proof
	// Axis is a type of axis against which the share proof is computed
	Axis rsmt2d.Axis
}

// Validate validates inclusion of the share under the given root CID.
func (s *ShareWithProof) Validate(rootHash []byte, x, y, edsSize int) bool {
	isParity := x >= edsSize/2 || y >= edsSize/2
	namespace := ParitySharesNamespace
	if !isParity {
		namespace = GetNamespace(s.Share)
	}
	return s.Proof.VerifyInclusion(
		sha256.New(), // TODO(@Wondertan): This should be defined somewhere globally
		namespace.ToNMT(),
		[][]byte{s.Share},
		rootHash,
	)
}

var tailPadding Share

func init() {
	shr := shares.TailPaddingShare()
	tailPadding = shr.ToBytes()
}
