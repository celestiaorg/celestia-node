package square

import (
	"github.com/celestiaorg/celestia-app/v3/pkg/appconsts"
	"github.com/celestiaorg/go-square/v2/share"
	"github.com/celestiaorg/nmt"
	"github.com/celestiaorg/rsmt2d"
)

// DefaultRSMT2DCodec sets the default rsmt2d.Codec for shares.
var DefaultRSMT2DCodec = appconsts.DefaultCodec

// MaxSquareSize is currently the maximum size supported for unerasured data in
// rsmt2d.ExtendedDataSquare.
var MaxSquareSize = appconsts.SquareSizeUpperBound(appconsts.LatestVersion)

// ShareWithProof contains data with corresponding Merkle Proof
type ShareWithProof struct { //nolint: revive
	// Share is a full data including namespace
	share.Share
	// Proof is a Merkle Proof of current share
	Proof *nmt.Proof
	// Axis is a type of axis against which the share proof is computed
	Axis rsmt2d.Axis
}

// Validate validates inclusion of the share under the given root CID.
func (s *ShareWithProof) Validate(rootHash []byte, x, y, edsSize int) bool {
	isParity := x >= edsSize/2 || y >= edsSize/2
	namespace := share.ParitySharesNamespace
	if !isParity {
		namespace = s.Share.Namespace()
	}
	return s.Proof.VerifyInclusion(
		NewSHA256Hasher(),
		namespace.Bytes(),
		[][]byte{s.Share.ToBytes()},
		rootHash,
	)
}
