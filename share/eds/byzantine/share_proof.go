package byzantine

import (
	"context"
	"crypto/sha256"
	"errors"

	"github.com/ipfs/boxo/blockservice"
	"github.com/ipfs/go-cid"
	logging "github.com/ipfs/go-log/v2"

	"github.com/celestiaorg/nmt"
	nmt_pb "github.com/celestiaorg/nmt/pb"
	"github.com/celestiaorg/rsmt2d"

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
	// Axis is a proof axis
	Axis rsmt2d.Axis
}

// Validate validates inclusion of the share under the given root CID.
func (s *ShareWithProof) Validate(root cid.Cid, x, y, edsSize int) bool {
	isParity := x >= edsSize/2 || y >= edsSize/2
	namespace := share.ParitySharesNamespace
	if !isParity {
		namespace = share.GetNamespace(s.Share)
	}
	return s.Proof.VerifyInclusion(
		sha256.New(), // TODO(@Wondertan): This should be defined somewhere globally
		namespace.ToNMT(),
		[][]byte{s.Share},
		ipld.NamespacedSha256FromCID(root),
	)
}

func (s *ShareWithProof) ShareWithProofToProto() *pb.Share {
	if s == nil {
		return &pb.Share{}
	}

	return &pb.Share{
		Data: s.Share,
		Proof: &nmt_pb.Proof{
			Start:                 int64(s.Proof.Start()),
			End:                   int64(s.Proof.End()),
			Nodes:                 s.Proof.Nodes(),
			LeafHash:              s.Proof.LeafHash(),
			IsMaxNamespaceIgnored: s.Proof.IsMaxNamespaceIDIgnored(),
		},
		ProofAxis: pb.Axis(s.Axis),
	}
}

// GetProofsForShares fetches Merkle proofs for the given shares
// and returns the result as an array of ShareWithProof.
func GetProofsForShares(
	ctx context.Context,
	bGetter blockservice.BlockGetter,
	root cid.Cid,
	shares [][]byte,
	axisType rsmt2d.Axis,
) ([]*ShareWithProof, error) {
	sharesWithProofs := make([]*ShareWithProof, len(shares))
	for index, share := range shares {
		if share != nil {
			proof, err := getProofsAt(ctx, bGetter, root, index, len(shares))
			if err != nil {
				return nil, err
			}
			sharesWithProofs[index] = &ShareWithProof{
				Share: share,
				Proof: &proof,
				Axis:  axisType,
			}
		}
	}
	return sharesWithProofs, nil
}

// getShareWithProof attempts to get a share with proof for the given share. It first tries to get a row proof
// and if that fails or proof is invalid, it tries to get a column proof.
func getShareWithProof(
	ctx context.Context,
	bGetter blockservice.BlockGetter,
	dah *share.Root,
	share share.Share,
	x, y int,
) (*ShareWithProof, error) {
	width := len(dah.RowRoots)
	// try row proofs
	root := dah.RowRoots[y]
	rootCid := ipld.MustCidFromNamespacedSha256(root)
	proof, err := getProofsAt(ctx, bGetter, rootCid, x, width)
	if err == nil {
		shareWithProof := &ShareWithProof{
			Share: share,
			Proof: &proof,
			Axis:  rsmt2d.Row,
		}
		if shareWithProof.Validate(rootCid, x, y, width) {
			return shareWithProof, nil
		}
	}

	// try column proofs
	root = dah.ColumnRoots[x]
	rootCid = ipld.MustCidFromNamespacedSha256(root)
	proof, err = getProofsAt(ctx, bGetter, rootCid, y, width)
	if err != nil {
		return nil, err
	}
	shareWithProof := &ShareWithProof{
		Share: share,
		Proof: &proof,
		Axis:  rsmt2d.Col,
	}
	if shareWithProof.Validate(rootCid, x, y, width) {
		return shareWithProof, nil
	}
	return nil, errors.New("failed to collect proof")
}

func getProofsAt(
	ctx context.Context,
	bGetter blockservice.BlockGetter,
	root cid.Cid,
	index,
	total int,
) (nmt.Proof, error) {
	proofPath := make([]cid.Cid, 0)
	proofPath, err := ipld.GetProof(ctx, bGetter, root, proofPath, index, total)
	if err != nil {
		return nmt.Proof{}, err
	}

	rangeProofs := make([][]byte, 0, len(proofPath))
	for i := len(proofPath) - 1; i >= 0; i-- {
		node := ipld.NamespacedSha256FromCID(proofPath[i])
		rangeProofs = append(rangeProofs, node)
	}

	return nmt.NewInclusionProof(index, index+1, rangeProofs, true), nil
}

func ProtoToShare(protoShares []*pb.Share) []*ShareWithProof {
	shares := make([]*ShareWithProof, len(protoShares))
	for i, share := range protoShares {
		if share.Proof == nil {
			continue
		}
		proof := ProtoToProof(share.Proof)
		shares[i] = &ShareWithProof{
			Share: share.Data,
			Proof: &proof,
			Axis:  rsmt2d.Axis(share.ProofAxis),
		}
	}
	return shares
}

func ProtoToProof(protoProof *nmt_pb.Proof) nmt.Proof {
	return nmt.NewInclusionProof(
		int(protoProof.Start),
		int(protoProof.End),
		protoProof.Nodes,
		protoProof.IsMaxNamespaceIgnored,
	)
}
