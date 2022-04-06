package fraud

import (
	"encoding"

	pb "github.com/celestiaorg/celestia-node/fraud/pb"
	"github.com/celestiaorg/celestia-node/service/header"
	"github.com/celestiaorg/nmt"
	"github.com/celestiaorg/rsmt2d"
)

type ProofType string

const (
	BadEncoding ProofType = "BadEncoding"
)

// Proof is a generic interface that will be used for all types of fraud proofs in the network.
type Proof interface {
	// Type returns the exact type of fraud proof
	Type() ProofType
	// Height returns header's height
	Height() uint64
	// Validate check the validity of fraud proof.
	// Validate throws an error if some conditions don't pass and thus fraud proof is not valid
	Validate(*header.ExtendedHeader, rsmt2d.Codec) error

	encoding.BinaryMarshaler
	encoding.BinaryUnmarshaler
}

func ProtoToProof(protoProof *pb.MerkleProof) nmt.Proof {
	return nmt.NewInclusionProof(int(protoProof.Start), int(protoProof.End), protoProof.Nodes, true)
}
