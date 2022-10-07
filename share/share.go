package share

import (
	"crypto/sha256"

	"github.com/ipfs/go-cid"
	logging "github.com/ipfs/go-log/v2"
	"github.com/tendermint/tendermint/pkg/consts"
	"go.opentelemetry.io/otel"

	"github.com/celestiaorg/celestia-node/share/ipld"
	"github.com/celestiaorg/celestia-node/share/pb"
	"github.com/celestiaorg/nmt"
	"github.com/celestiaorg/nmt/namespace"
)

var (
	log    = logging.Logger("share")
	tracer = otel.Tracer("share")

	// DefaultRSMT2DCodec sets the default rsmt2d.Codec for shares.
	DefaultRSMT2DCodec = consts.DefaultCodec
)

const (
	// MaxSquareSize is currently the maximum size supported for unerasured data in rsmt2d.ExtendedDataSquare.
	MaxSquareSize = consts.MaxSquareSize
	// NamespaceSize is a system-wide size for NMT namespaces.
	NamespaceSize = consts.NamespaceSize
	// Size is a system-wide size of a share, including both data and namespace ID
	Size = consts.ShareSize
)

// Share contains the raw share data without the corresponding namespace.
// NOTE: Alias for the byte is chosen to keep maximal compatibility, especially with rsmt2d. Ideally, we should define
// reusable type elsewhere and make everyone(Core, rsmt2d, ipld) to rely on it.
type Share = []byte

// ID gets the namespace ID from the share.
func ID(s Share) namespace.ID {
	return s[:NamespaceSize]
}

// Data gets data from the share.
func Data(s Share) []byte {
	return s[NamespaceSize:]
}

// ShareWithProof contains data with corresponding Merkle Proof
type ShareWithProof struct { //nolint:revive
	// Share is a full data including namespace
	Share
	// Proof is a Merkle Proof of current share
	Proof *nmt.Proof
}

// NewShareWithProof takes the given leaf and its path, starting from the tree root,
// and computes the nmt.Proof for it.
func NewShareWithProof(index int, share Share, pathToLeaf []cid.Cid) *ShareWithProof {
	rangeProofs := make([][]byte, 0, len(pathToLeaf))
	for i := len(pathToLeaf) - 1; i >= 0; i-- {
		node := ipld.NamespacedSha256FromCID(pathToLeaf[i])
		rangeProofs = append(rangeProofs, node)
	}

	proof := nmt.NewInclusionProof(index, index+1, rangeProofs, true)
	return &ShareWithProof{
		share,
		&proof,
	}
}

// Validate validates inclusion of the share under the given root CID.
func (s *ShareWithProof) Validate(root cid.Cid) bool {
	return s.Proof.VerifyInclusion(
		sha256.New(), // TODO(@Wondertan): This should be defined somewhere globally
		ID(s.Share),
		[][]byte{Data(s.Share)},
		ipld.NamespacedSha256FromCID(root),
	)
}

func (s *ShareWithProof) ShareWithProofToProto() *pb.Share {
	return &pb.Share{
		Data: s.Share,
		Proof: &pb.MerkleProof{
			Start:    int64(s.Proof.Start()),
			End:      int64(s.Proof.End()),
			Nodes:    s.Proof.Nodes(),
			LeafHash: s.Proof.LeafHash(),
		},
	}
}

func ProtoToShare(protoShares []*pb.Share) []*ShareWithProof {
	shares := make([]*ShareWithProof, len(protoShares))
	for i, share := range protoShares {
		proof := ProtoToProof(share.Proof)
		shares[i] = &ShareWithProof{share.Data, &proof}
	}
	return shares
}

func ProtoToProof(protoProof *pb.MerkleProof) nmt.Proof {
	return nmt.NewInclusionProof(int(protoProof.Start), int(protoProof.End), protoProof.Nodes, true)
}
