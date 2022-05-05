package fraud

import (
	"context"
)

type Service interface {
	Start(context.Context) error
	Stop(context.Context) error

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
// subscribe/unsubscribe from new FraudProofs events from the
// network.
type Subscriber interface {
	// Subscribe allows to subscribe on pub sub topic by its type.
	// Subscribe should register pub-sub validator on topic.
	Subscribe(ProofType) (Subscription, error)
	// RegisterUnmarshaler registers unmarshaler for the given ProofType.
	// If there is no unmarshaler for `ProofType`, then `Subscribe` returns an error.
	RegisterUnmarshaler(ProofType, proofUnmarshaler) error
	// UnregisterUnmarshaler removes unmarshaler for the given ProofType.
	// If there is no unmarshaler for `ProofType`, then it returns an error.
	UnregisterUnmarshaler(ProofType) error

	// AddValidator adds internal validation to topic inside libp2p
	AddValidator(ProofType, Validator) error
}

// Subscription returns a valid proof if one is received on the topic.
type Subscription interface {
	// Proof returns already verified valid proof
	Proof(context.Context) (Proof, error)
	Cancel()
}
