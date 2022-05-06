package header

import (
	"context"
	"errors"
	"fmt"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	tmbytes "github.com/tendermint/tendermint/libs/bytes"
)

// Validator aliases a func that validates ExtendedHeader.
type Validator = func(context.Context, *ExtendedHeader) pubsub.ValidationResult

// Subscriber encompasses the behavior necessary to
// subscribe/unsubscribe from new ExtendedHeader events from the
// network.
type Subscriber interface {
	// Subscribe creates long-living Subscription for validated ExtendedHeaders.
	// Multiple Subscriptions can be created.
	Subscribe() (Subscription, error)
	// AddValidator registers a Validator for all Subscriptions.
	// Registered Validators screen ExtendedHeaders for their validity
	// before they are sent through Subscriptions.
	// Multiple validators can be registered.
	AddValidator(Validator) error
}

// Subscription can retrieve the next ExtendedHeader from the
// network.
type Subscription interface {
	// NextHeader returns the newest verified and valid ExtendedHeader
	// in the network.
	NextHeader(ctx context.Context) (*ExtendedHeader, error)
	// Cancel cancels the subscription.
	Cancel()
}

// Broadcaster broadcasts an ExtendedHeader to the network.
type Broadcaster interface {
	Broadcast(ctx context.Context, header *ExtendedHeader, opts ...pubsub.PubOpt) error
}

// Exchange encompasses the behavior necessary to request ExtendedHeaders
// from the network.
type Exchange interface {
	// RequestHead requests the latest ExtendedHeader. Note that the ExtendedHeader
	// must be verified thereafter.
	RequestHead(ctx context.Context) (*ExtendedHeader, error)
	// RequestHeader performs a request for the ExtendedHeader at the given
	// height to the network. Note that the ExtendedHeader must be verified
	// thereafter.
	RequestHeader(ctx context.Context, height uint64) (*ExtendedHeader, error)
	// RequestHeaders performs a request for the given range of ExtendedHeaders
	// to the network. Note that the ExtendedHeaders must be verified thereafter.
	RequestHeaders(ctx context.Context, origin, amount uint64) ([]*ExtendedHeader, error)
	// RequestByHash performs a request for the ExtendedHeader by the given hash corresponding
	// to the RawHeader. Note that the ExtendedHeader must be verified thereafter.
	RequestByHash(ctx context.Context, hash tmbytes.HexBytes) (*ExtendedHeader, error)
}

var (
	// ErrNotFound is returned when there is no requested header.
	ErrNotFound = errors.New("header: not found")

	// ErrNoHead is returned when Store is empty (does not contain any known header).
	ErrNoHead = fmt.Errorf("header/store: no chain head")

	// ErrNonAdjacent is returned when Store is appended with a header not adjacent to the stored head.
	ErrNonAdjacent = fmt.Errorf("header/store: non-adjacent")
)

// Store encompasses the behavior necessary to store and retrieve ExtendedHeaders
// from a node's local storage.
type Store interface {
	// Start starts the store.
	Start(context.Context) error

	// Stop stops the store by preventing further writes
	// and waiting till the ongoing ones are done.
	Stop(context.Context) error

	// Init initializes Store with the given head, meaning it is initialized with the genesis header.
	Init(context.Context, *ExtendedHeader) error

	// Height reports current height of the chain head.
	Height() uint64

	// Head returns the ExtendedHeader of the chain head.
	Head(context.Context) (*ExtendedHeader, error)

	// Get returns the ExtendedHeader corresponding to the given hash.
	Get(context.Context, tmbytes.HexBytes) (*ExtendedHeader, error)

	// GetByHeight returns the ExtendedHeader corresponding to the given block height.
	GetByHeight(context.Context, uint64) (*ExtendedHeader, error)

	// GetRangeByHeight returns the given range [from:to) of ExtendedHeaders.
	GetRangeByHeight(ctx context.Context, from, to uint64) ([]*ExtendedHeader, error)

	// Has checks whether ExtendedHeader is already stored.
	Has(context.Context, tmbytes.HexBytes) (bool, error)

	// Append stores and verifies the given ExtendedHeader(s).
	// It requires them to be adjacent and in ascending order,
	// as it applies them contiguously on top of the current head height.
	// It returns the amount of successfully applied headers,
	// so caller can understand what given header was invalid, if any.
	Append(context.Context, ...*ExtendedHeader) (int, error)
}
