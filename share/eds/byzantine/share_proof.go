package byzantine

import (
	"context"
	"errors"

	"github.com/ipfs/boxo/blockservice"
	logging "github.com/ipfs/go-log/v2"

	libshare "github.com/celestiaorg/go-square/v2/share"
	"github.com/celestiaorg/nmt"
	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/ipld"
)

var log = logging.Logger("share/byzantine")

// ShareWithProof contains data with corresponding Merkle Proof
type ShareWithProof struct {
	// Share is a full data including namespace
	libshare.Share
	// Proof is a Merkle Proof of current share
	Proof *nmt.Proof
	// Axis is a proof axis
	Axis rsmt2d.Axis
}

// Validate validates inclusion of the share under the given root CID.
func (s *ShareWithProof) Validate(roots *share.AxisRoots, axisType rsmt2d.Axis, axisIdx, shrIdx int) bool {
	var rootHash []byte
	switch axisType {
	case rsmt2d.Row:
		rootHash = rootHashForCoordinates(roots, s.Axis, shrIdx, axisIdx)
	case rsmt2d.Col:
		rootHash = rootHashForCoordinates(roots, s.Axis, axisIdx, shrIdx)
	}

	edsSize := len(roots.RowRoots)
	isParity := shrIdx >= edsSize/2 || axisIdx >= edsSize/2
	namespace := libshare.ParitySharesNamespace
	if !isParity {
		namespace = s.Namespace()
	}
	return s.Proof.VerifyInclusion(
		share.NewSHA256Hasher(),
		namespace.Bytes(),
		[][]byte{s.ToBytes()},
		rootHash,
	)
}

// GetShareWithProof attempts to get a share with proof for the given share. It first tries to get
// a row proof and if that fails or proof is invalid, it tries to get a column proof.
func GetShareWithProof(
	ctx context.Context,
	bGetter blockservice.BlockGetter,
	roots *share.AxisRoots,
	share libshare.Share,
	axisType rsmt2d.Axis, axisIdx, shrIdx int,
) (*ShareWithProof, error) {
	if axisType == rsmt2d.Col {
		axisIdx, shrIdx, axisType = shrIdx, axisIdx, rsmt2d.Row
	}
	width := len(roots.RowRoots)
	// try row proofs
	root := roots.RowRoots[axisIdx]
	proof, err := ipld.GetProof(ctx, bGetter, root, shrIdx, width)
	if err == nil {
		shareWithProof := &ShareWithProof{
			Share: share,
			Proof: &proof,
			Axis:  rsmt2d.Row,
		}
		if shareWithProof.Validate(roots, axisType, axisIdx, shrIdx) {
			return shareWithProof, nil
		}
	}

	// try column proofs
	root = roots.ColumnRoots[shrIdx]
	proof, err = ipld.GetProof(ctx, bGetter, root, axisIdx, width)
	if err != nil {
		return nil, err
	}
	shareWithProof := &ShareWithProof{
		Share: share,
		Proof: &proof,
		Axis:  rsmt2d.Col,
	}
	if shareWithProof.Validate(roots, axisType, axisIdx, shrIdx) {
		return shareWithProof, nil
	}
	return nil, errors.New("failed to collect proof")
}

func rootHashForCoordinates(r *share.AxisRoots, axisType rsmt2d.Axis, x, y int) []byte {
	if axisType == rsmt2d.Row {
		return r.RowRoots[y]
	}
	return r.ColumnRoots[x]
}
