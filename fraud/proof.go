package fraud

import (
	"context"
	"encoding"
	"fmt"

	"github.com/celestiaorg/celestia-node/header"
)

type ErrFraudExists struct {
	Proof []Proof
}

func (e *ErrFraudExists) Error() string {
	return fmt.Sprintf("fraud: %s proof exists\n", e.Proof[0].Type())
}

type errNoUnmarshaler struct {
	proofType ProofType
}

func (e *errNoUnmarshaler) Error() string {
	return fmt.Sprintf("fraud: unmarshaler for %s type is not registered", e.proofType)
}

type ProofType string

const (
	BadEncoding ProofType = "badencoding"
)

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

// OnProof subscribes to the given Fraud Proof topic via the given Subscriber.
// In case a Fraud Proof is received, then the given handle function will be invoked.
func OnProof(ctx context.Context, subscriber Subscriber, p ProofType, handle func(proof Proof)) {
	subscription, err := subscriber.Subscribe(p)
	if err != nil {
		log.Error(err)
		return
	}
	defer subscription.Cancel()

	// At this point we receive already verified fraud proof,
	// so there is no need to call Validate.
	proof, err := subscription.Proof(ctx)
	if err != nil {
		if err != context.Canceled {
			log.Errorw("reading next proof failed", "err", err)
		}
		return
	}

	handle(proof)
}

// Unmarshal converts raw bytes into respective Proof type.
func Unmarshal(proofType ProofType, msg []byte) (Proof, error) {
	unmarshalersLk.RLock()
	defer unmarshalersLk.RUnlock()
	unmarshaler, ok := defaultUnmarshalers[proofType]
	if !ok {
		return nil, &errNoUnmarshaler{proofType: proofType}
	}
	return unmarshaler(msg)
}
