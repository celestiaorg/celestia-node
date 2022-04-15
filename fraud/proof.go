package fraud

import (
	"encoding"

	pb "github.com/celestiaorg/celestia-node/fraud/pb"
	"github.com/celestiaorg/celestia-node/service/header"
	"github.com/celestiaorg/nmt"
)

type ProofType int

const (
	BadEncoding ProofType = iota
)

// Proof is a generic interface that will be used for all types of fraud proofs in the network.
type Proof interface {
	// Type returns the exact type of fraud proof
	Type() ProofType
	// Height returns the block height corresponding to the Proof
	Height() uint64
	// Validate check the validity of fraud proof.
	// Validate throws an error if some conditions don't pass and thus fraud proof is not valid
	Validate(*header.ExtendedHeader) error

	encoding.BinaryMarshaler
	encoding.BinaryUnmarshaler
}

func ProtoToProof(protoProof *pb.MerkleProof) nmt.Proof {
	return nmt.NewInclusionProof(int(protoProof.Start), int(protoProof.End), protoProof.Nodes, true)
}
