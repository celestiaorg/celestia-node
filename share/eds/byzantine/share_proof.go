package byzantine

import (
	"context"

	"github.com/ipfs/go-blockservice"
	"github.com/ipfs/go-cid"
	logging "github.com/ipfs/go-log/v2"
	"github.com/minio/sha256-simd"

	"github.com/celestiaorg/nmt"

	"github.com/celestiaorg/celestia-node/share"
	pb "github.com/celestiaorg/celestia-node/share/eds/byzantine/pb"
	"github.com/celestiaorg/celestia-node/share/ipld"
)

var log = logging.Logger("share/byzantine")

// ShareWithProof contains data with corresponding Merkle Proof
type ShareWithProof struct {
	// Share is a full data including namespace
	share.Share
	// Proof is a Merkle Proof of current share
	Proof *nmt.Proof
}

// NewShareWithProof takes the given leaf and its path, starting from the tree root,
// and computes the nmt.Proof for it.
func NewShareWithProof(index int, share share.Share, pathToLeaf []cid.Cid) *ShareWithProof {
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
		share.GetNamespace(s.Share).ToNMT(),
		[][]byte{share.GetData(s.Share)},
		ipld.NamespacedSha256FromCID(root),
	)
}

func (s *ShareWithProof) ShareWithProofToProto() *pb.Share {
	if s == nil {
		return &pb.Share{}
	}

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

// GetProofsForShares fetches Merkle proofs for the given shares
// and returns the result as an array of ShareWithProof.
func GetProofsForShares(
	ctx context.Context,
	bGetter blockservice.BlockGetter,
	root cid.Cid,
	shares [][]byte,
) ([]*ShareWithProof, error) {
	proofs := make([]*ShareWithProof, len(shares))
	for index, share := range shares {
		if share != nil {
			proof, err := getProofsAt(ctx, bGetter, root, index, len(shares))
			if err != nil {
				return nil, err
			}
			proofs[index] = proof
		}
	}
	return proofs, nil
}

func getProofsAt(
	ctx context.Context,
	bGetter blockservice.BlockGetter,
	root cid.Cid,
	index,
	total int,
) (*ShareWithProof, error) {
	proof := make([]cid.Cid, 0)
	// TODO(@vgonkivs): Combine GetLeafData and GetProof in one function as the are traversing the same
	// tree. Add options that will control what data will be fetched.
	node, err := ipld.GetLeaf(ctx, bGetter, root, index, total)
	if err != nil {
		return nil, err
	}

	proof, err = ipld.GetProof(ctx, bGetter, root, proof, index, total)
	if err != nil {
		return nil, err
	}
	return NewShareWithProof(index, node.RawData(), proof), nil
}

func ProtoToShare(protoShares []*pb.Share) []*ShareWithProof {
	shares := make([]*ShareWithProof, len(protoShares))
	for i, share := range protoShares {
		if share.Proof == nil {
			continue
		}
		proof := ProtoToProof(share.Proof)
		shares[i] = &ShareWithProof{share.Data, &proof}
	}
	return shares
}

func ProtoToProof(protoProof *pb.MerkleProof) nmt.Proof {
	return nmt.NewInclusionProof(int(protoProof.Start), int(protoProof.End), protoProof.Nodes, ipld.NMTIgnoreMaxNamespace)
}
