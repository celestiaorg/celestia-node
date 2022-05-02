package fraud

import (
	"encoding"
	"fmt"

	"github.com/celestiaorg/celestia-node/service/header"
)

type ProofType int

const (
	BadEncoding ProofType = iota
)

func (p ProofType) String() string {
	switch p {
	case BadEncoding:
		return "BadEncoding"
	default:
		panic(fmt.Sprintf("invalid proof type: %d", p))
	}
}

// Proof is a generic interface that will be used for all types of fraud proofs in the network.
type Proof interface {
	// Type returns the exact type of fraud proof
	Type() ProofType
	// Height returns the block height corresponding to the Proof
	Height() uint64
	// Validate check the validity of fraud proof.
	// Validate throws an error if some conditions don't pass and thus fraud proof is not valid
	// NOTE: header.ExtendedHeader should pass basic validation otherwise it will panic if it's malformed
	Validate(*header.ExtendedHeader) error

	encoding.BinaryMarshaler
	encoding.BinaryUnmarshaler
}
