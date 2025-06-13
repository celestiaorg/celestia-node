package proof

import (
	"github.com/celestiaorg/nmt"
	nmt_pb "github.com/celestiaorg/nmt/pb"

	proof_pb "github.com/celestiaorg/celestia-node/share/proof/pb"
)

// IncompleteRowsProof contains proofs for the boundary rows of a data range.
// Since all intermediate rows between the first and last row are complete,
// they can be easily recomputed and don't need to be stored. This helps to
// reduce proof size by only keeping proofs for the rows that can't be
// recomputed.
type IncompleteRowsProof struct {
	FirstIncompleteRowProof *nmt.Proof `json:"first_row_proof,omitempty"`
	LastIncompleteRowProof  *nmt.Proof `json:"last_row_proof,omitempty"`
}

func (p *IncompleteRowsProof) ToProto() *proof_pb.IncompleteRowsProof {
	if p == nil {
		return nil
	}
	return &proof_pb.IncompleteRowsProof{
		FirstIncompleteRowProof: nmtToNmtPbProof(p.FirstIncompleteRowProof),
		LastIncompleteRowProof:  nmtToNmtPbProof(p.LastIncompleteRowProof),
	}
}

func IncompleteRowsProofFromProto(p *proof_pb.IncompleteRowsProof) *IncompleteRowsProof {
	if p == nil {
		return nil
	}
	return &IncompleteRowsProof{
		FirstIncompleteRowProof: pbNmtToNmtProof(p.FirstIncompleteRowProof),
		LastIncompleteRowProof:  pbNmtToNmtProof(p.LastIncompleteRowProof),
	}
}

func nmtToNmtPbProof(proof *nmt.Proof) *nmt_pb.Proof {
	if proof == nil {
		return nil
	}
	return &nmt_pb.Proof{
		Start:                 int64(proof.Start()),
		End:                   int64(proof.End()),
		Nodes:                 proof.Nodes(),
		LeafHash:              proof.LeafHash(),
		IsMaxNamespaceIgnored: proof.IsMaxNamespaceIDIgnored(),
	}
}

func pbNmtToNmtProof(pbproof *nmt_pb.Proof) *nmt.Proof {
	if pbproof == nil {
		return nil
	}
	proof := nmt.ProtoToProof(*pbproof)
	return &proof
}
