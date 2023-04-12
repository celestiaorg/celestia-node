package fraud

import (
	"context"

	"github.com/celestiaorg/go-header"
)

// HeaderFetcher aliases a function that is used to fetch an ExtendedHeader from store by height.
type HeaderFetcher func(context.Context, uint64) (header.Header, error)

// ProofUnmarshaler aliases a function that parses data to `Proof`.
type ProofUnmarshaler func([]byte) (Proof, error)

// Service encompasses the behavior necessary to subscribe and broadcast
// fraud proofs within the network.
type Service interface {
	Subscriber
	Broadcaster
	Getter
}

// Broadcaster is a generic interface that sends a `Proof` to all nodes subscribed on the
// Broadcaster's topic.
type Broadcaster interface {
	// Broadcast takes a fraud `Proof` data structure interface and broadcasts it to local
	// subscriptions and peers. It may additionally cache/persist Proofs for future
	// access via Getter and to serve Proof requests to peers in the network.
	Broadcast(context.Context, Proof) error
}

// Subscriber encompasses the behavior necessary to
// subscribe/unsubscribe from new FraudProof events from the
// network.
type Subscriber interface {
	// Subscribe allows to subscribe on a Proof pub sub topic by its type.
	Subscribe(ProofType) (Subscription, error)
}

// Getter encompasses the behavior to fetch stored fraud proofs.
type Getter interface {
	// Get fetches fraud proofs from the disk by its type.
	Get(context.Context, ProofType) ([]Proof, error)
}

// Subscription returns a valid proof if one is received on the topic.
type Subscription interface {
	// Proof returns already verified valid proof.
	Proof(context.Context) (Proof, error)
	Cancel()
}
