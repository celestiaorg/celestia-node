package ipld

import (
	"crypto/sha256"

	"github.com/ipfs/go-cid"
	"github.com/tendermint/tendermint/pkg/consts"

	"github.com/celestiaorg/celestia-node/ipld/pb"
	"github.com/celestiaorg/celestia-node/ipld/plugin"
	"github.com/celestiaorg/nmt"
	"github.com/celestiaorg/nmt/namespace"
)

const (
	// MaxSquareSize is currently the maximum size supported for unerasured data in rsmt2d.ExtendedDataSquare.
	MaxSquareSize = consts.MaxSquareSize
	// NamespaceSize is a system-wide size for NMT namespaces.
	NamespaceSize = consts.NamespaceSize
	// ShareSize is a system-wide size of a share, including both data and namespace ID
	ShareSize = consts.ShareSize
)

// DefaultRSMT2DCodec sets the default rsmt2d.Codec for shares.
var DefaultRSMT2DCodec = consts.DefaultCodec

// Share contains the raw share data without the corresponding namespace.
// NOTE: Alias for the byte is chosen to keep maximal compatibility, especially with rsmt2d. Ideally, we should define
// reusable type elsewhere and make everyone(Core, rsmt2d, ipld) to rely on it.
type Share = []byte

// ShareID gets the namespace ID from the share.
func ShareID(s Share) namespace.ID {
	return s[:NamespaceSize]
}

// ShareData gets data from the share.
func ShareData(s Share) []byte {
	return s[NamespaceSize:]
}

// ShareWithProof contains data with corresponding Merkle Proof
type ShareWithProof struct {
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
		node := plugin.NamespacedSha256FromCID(pathToLeaf[i])
		rangeProofs = append(rangeProofs, node)
	}

	proof := nmt.NewInclusionProof(index, index+1, rangeProofs, true)
	return &ShareWithProof{
		share,
		&proof,
	}
}

func (s *ShareWithProof) Validate(root []byte) bool {
	// As nmt prepends NamespaceID twice, we need to pass the full data with NamespaceID in VerifyInclusion
	return s.Proof.VerifyInclusion(sha256.New(), ShareID(s.Share), ShareData(s.Share), root)
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
