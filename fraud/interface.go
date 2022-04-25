package fraud

import "context"

type Service interface {
	Subscriber
	Broadcaster
}

// Broadcaster is a generic interface that sends a `Proof` to all nodes subscribed on the Broadcaster's topic.
type Broadcaster interface {
	// Broadcast takes a fraud `Proof` data structure that implements standard BinaryMarshal
	// interface and broadcasts it to all subscribed peers.
	Broadcast(ctx context.Context, p Proof) error
}

// Subscriber encompasses the behavior necessary to
// subscribe/unsubscribe from new FraudProofs events from the
// network.
type Subscriber interface {
	// Subscribe allows to subscribe on pub sub topic by it's type.
	// Subscribe should register pub-sub validator on topic.
	Subscribe(proofType ProofType) (Subscription, error)
	// RegisterUnmarshaller registers unmarshaller for the given ProofType.
	// If there is no umarshaller for `ProofType`, then `Subscribe` returns an error.
	RegisterUnmarshaller(proofType ProofType, f proofUnmarshaller) error
	// UnregisterUnmarshaller removes unmarshaller for the given ProofType.
	// If there is no unmarshaller for `ProofType`, then it returns an error.
	UnregisterUnmarshaller(proofType ProofType) error

	// AddValidator adds internal validation to topic inside libp2p
	AddValidator(proofType ProofType, fetcher headerFetcher) error
}

// Subscription returns a valid proof if one is received on the topic.
type Subscription interface {
	// Proof returns already verified valid proof
	Proof(context.Context) (Proof, error)
	Cancel()
}
