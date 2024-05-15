package byzantine

import (
	"context"
	"errors"
	"math"

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
func (s *ShareWithProof) Validate(dah *share.Root, axisType rsmt2d.Axis, axisIdx, shrIdx int) bool {
	var rootHash []byte
	switch axisType {
	case rsmt2d.Row:
		rootHash = rootHashForCoordinates(dah, s.Axis, shrIdx, axisIdx)
	case rsmt2d.Col:
		rootHash = rootHashForCoordinates(dah, s.Axis, axisIdx, shrIdx)
	}

	edsSize := len(dah.RowRoots)
	isParity := shrIdx >= edsSize/2 || axisIdx >= edsSize/2
	namespace := share.ParitySharesNamespace
	if !isParity {
		namespace = share.GetNamespace(s.Share)
	}
	return s.Proof.VerifyInclusion(
		share.NewSHA256Hasher(),
		namespace.ToNMT(),
		[][]byte{s.Share},
		rootHash,
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

// GetShareWithProof attempts to get a share with proof for the given share. It first tries to get a row proof
// and if that fails or proof is invalid, it tries to get a column proof.
func GetShareWithProof(
	ctx context.Context,
	bGetter blockservice.BlockGetter,
	dah *share.Root,
	share share.Share,
	axisType rsmt2d.Axis, axisIdx, shrIdx int,
) (*ShareWithProof, error) {
	if axisType == rsmt2d.Col {
		axisIdx, shrIdx, axisType = shrIdx, axisIdx, rsmt2d.Row
	}
	width := len(dah.RowRoots)
	// try row proofs
	root := dah.RowRoots[axisIdx]
	rootCid := ipld.MustCidFromNamespacedSha256(root)
	proof, err := getProofsAt(ctx, bGetter, rootCid, shrIdx, width)
	if err == nil {
		shareWithProof := &ShareWithProof{
			Share: share,
			Proof: &proof,
			Axis:  rsmt2d.Row,
		}
		if shareWithProof.Validate(dah, axisType, axisIdx, shrIdx) {
			return shareWithProof, nil
		}
	}

	// try column proofs
	root = dah.ColumnRoots[shrIdx]
	rootCid = ipld.MustCidFromNamespacedSha256(root)
	proof, err = getProofsAt(ctx, bGetter, rootCid, axisIdx, width)
	if err != nil {
		return nil, err
	}
	shareWithProof := &ShareWithProof{
		Share: share,
		Proof: &proof,
		Axis:  rsmt2d.Col,
	}
	if shareWithProof.Validate(dah, axisType, axisIdx, shrIdx) {
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
	proofPath := make([]cid.Cid, 0, int(math.Sqrt(float64(total))))
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

func rootHashForCoordinates(r *share.Root, axisType rsmt2d.Axis, x, y int) []byte {
	if axisType == rsmt2d.Row {
		return r.RowRoots[y]
	}
	return r.ColumnRoots[x]
}
