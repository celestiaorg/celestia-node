package fraud

import (
	"context"
	"encoding"
	"fmt"

	"github.com/celestiaorg/celestia-node/header"
)

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

// onFraudProof listens to Fraud Proof and stops services immediately if it is received.
func onFraudProof(ctx context.Context, s Subscriber, p ProofType, stop func(context.Context) error) {
	var err error
	defer func() {
		if err == context.Canceled {
			return
		}
		stopErr := stop(ctx)
		if stopErr != nil {
			log.Warn(stopErr)
		}
	}()

	subscription, err := s.Subscribe(p)
	if err != nil {
		log.Errorw("failed to subscribe to fraud proof", "fraudProof", p, "err", err)
		return
	}
	defer subscription.Cancel()

	// At this point we receive already verified fraud proof,
	// so there are no needs to call Validate.
	_, err = subscription.Proof(ctx)
	if err != nil {
		if err == context.Canceled {
			return
		}
		log.Errorw("reading next proof failed", "err", err)
	}
}
