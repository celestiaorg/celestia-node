package fraud

import (
	"encoding"
	"errors"
	"fmt"

	"github.com/celestiaorg/celestia-node/header"
)

var ErrProofNotFound = errors.New("fraud: proof was not found")

type ErrFraudExists struct {
	Proof []Proof
}

func (e *ErrFraudExists) Error() string {
	return fmt.Sprintf("fraud: %s proof exists\n", e.Proof[0].Type())
}

type ProofType int

const (
	BadEncoding ProofType = iota
)

func (p ProofType) String() string {
	switch p {
	case BadEncoding:
		return "badencoding"
	default:
		panic(fmt.Sprintf("fraud: invalid proof type: %d", p))
	}
}

func toProof(p int32) ProofType {
	switch p {
	case 0:
		return BadEncoding
	default:
		panic(fmt.Sprintf("fraud: invalid proof type: %d", p))
	}
}

// Proof is a generic interface that will be used for all types of fraud proofs in the network.
type Proof interface {
	// Type returns the exact type of fraud proof.
	Type() ProofType
	// HeaderHash returns the block hash.
	HeaderHash() []byte
	// Height returns the block height corresponding to the Proof.
	Height() uint64
	// Validate check the validity of fraud proof.
	// Validate throws an error if some conditions don't pass and thus fraud proof is not valid.
	// NOTE: header.ExtendedHeader should pass basic validation otherwise it will panic if it's malformed.
	Validate(*header.ExtendedHeader) error

	encoding.BinaryMarshaler
	encoding.BinaryUnmarshaler
}
