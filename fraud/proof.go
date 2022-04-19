package fraud

import (
	"encoding"

	"github.com/celestiaorg/celestia-node/service/header"
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
