package fraud

import (
	"context"
)

// ProofUnmarshaler aliases a function that parses data to `Proof`.
type ProofUnmarshaler func([]byte) (Proof, error)

// Service encompasses the behavior necessary to subscribe and broadcast
// Fraud Proofs within the network.
type Service interface {
	Subscriber
	Broadcaster
}

// Broadcaster is a generic interface that sends a `Proof` to all nodes subscribed on the Broadcaster's topic.
type Broadcaster interface {
	// Broadcast takes a fraud `Proof` data structure that implements standard BinaryMarshal
	// interface and broadcasts it to all subscribed peers.
	Broadcast(context.Context, Proof) error
}

// Subscriber encompasses the behavior necessary to
// subscribe/unsubscribe from new FraudProof events from the
// network.
type Subscriber interface {
	// Subscribe allows to subscribe on a Proof pub sub topic by its type.
	Subscribe(ProofType) (Subscription, error)
	// RegisterUnmarshaler registers unmarshaler for the given ProofType.
	// If there is no unmarshaler for `ProofType`, then `Subscribe` returns an error.
	RegisterUnmarshaler(ProofType, ProofUnmarshaler) error
	// UnregisterUnmarshaler removes unmarshaler for the given ProofType.
	// If there is no unmarshaler for `ProofType`, then it returns an error.
	UnregisterUnmarshaler(ProofType) error
}

// Subscription returns a valid proof if one is received on the topic.
type Subscription interface {
	// Proof returns already verified valid proof.
	Proof(context.Context) (Proof, error)
	Cancel()
}
