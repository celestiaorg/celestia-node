package ipld

import (
	"crypto/sha256"

	"github.com/ipfs/go-cid"
	"github.com/tendermint/tendermint/pkg/consts"

	pb "github.com/celestiaorg/celestia-node/ipld/pb"
	"github.com/celestiaorg/celestia-node/ipld/plugin"
	"github.com/celestiaorg/nmt"
	"github.com/celestiaorg/nmt/namespace"
)

const (
	// MaxSquareSize is currently the maximum size supported for unerasured data in rsmt2d.ExtendedDataSquare.
	MaxSquareSize = 128
	// NamespaceSize is a system wide size for NMT namespaces.
	// TODO(Wondertan): Should be part of IPLD/NMT plugin
	NamespaceSize = 8
)

// TODO(Wondertan):
//  Currently Share prepends namespace bytes while NamespaceShare just takes a copy of namespace
//  separating it in separate field. This is really confusing for newcomers and even for those who worked with code,
//  but had some time off of it. Instead, we shouldn't copy(1) and likely have only one type - NamespacedShare, as we
//  don't support shares without namespace.

// Share contains the raw share data without the corresponding namespace.
type Share []byte

// TODO(Wondertan): Consider using alias to namespace.PrefixedData instead
// NamespacedShare extends a Share with the corresponding namespace.
type NamespacedShare struct {
	Share
	ID namespace.ID
}

func (n NamespacedShare) NamespaceID() namespace.ID {
	return n.ID
}

func (n NamespacedShare) Data() []byte {
	return n.Share
}

// NamespacedShares is just a list of NamespacedShare elements.
// It can be used to extract the raw shares.
type NamespacedShares []NamespacedShare

// Raw returns the raw shares that can be fed into the erasure coding
// library (e.g. rsmt2d).
func (ns NamespacedShares) Raw() [][]byte {
	res := make([][]byte, len(ns))
	for i, nsh := range ns {
		res[i] = nsh.Share
	}
	return res
}

type NamespacedShareWithProof struct {
	ID namespace.ID
	Share
	Proof *nmt.Proof
}

// NewShareWithProof takes the given leaf and its path, starting from the tree root,
// and computes the nmt.Proof for it.
func NewShareWithProof(index int, leaf []byte, pathToLeaf []cid.Cid) *NamespacedShareWithProof {
	rangeProofs := make([][]byte, 0)
	for idx := len(pathToLeaf) - 1; idx >= 0; idx-- {
		node := plugin.NamespacedSha256FromCID(pathToLeaf[idx])
		rangeProofs = append(rangeProofs, node)
	}

	id := namespace.ID(leaf[:consts.NamespaceSize])
	proof := nmt.NewInclusionProof(index, index+1, rangeProofs, true)
	return &NamespacedShareWithProof{id, leaf[consts.NamespaceSize:], &proof}
}

func (s *NamespacedShareWithProof) Validate(root []byte) bool {
	// As nmt prepends NamespaceID twice, we need to pass the full data with NamespaceID in
	// VerifyInclusion

	return s.Proof.VerifyInclusion(sha256.New(), s.ID, s.Share, root)
}

func (s *NamespacedShareWithProof) ShareWithProofToProto() *pb.Share {
	return &pb.Share{
		NamespaceID: s.ID,
		Data:        s.Share,
		Proof: &pb.MerkleProof{
			Start:    int64(s.Proof.Start()),
			End:      int64(s.Proof.End()),
			Nodes:    s.Proof.Nodes(),
			LeafHash: s.Proof.LeafHash(),
		},
	}
}

func ProtoToShare(protoShares []*pb.Share) []*NamespacedShareWithProof {
	shares := make([]*NamespacedShareWithProof, len(protoShares))
	for _, share := range protoShares {
		proof := ProtoToProof(share.Proof)
		shares = append(
			shares,
			&NamespacedShareWithProof{share.NamespaceID, share.Data, &proof},
		)
	}
	return shares
}

func ProtoToProof(protoProof *pb.MerkleProof) nmt.Proof {
	return nmt.NewInclusionProof(int(protoProof.Start), int(protoProof.End), protoProof.Nodes, true)
}
