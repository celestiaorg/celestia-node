package header

import (
	"context"
	"errors"
	"fmt"

	tmbytes "github.com/tendermint/tendermint/libs/bytes"
)

// Subscriber encompasses the behavior necessary to
// subscribe/unsubscribe from new ExtendedHeader events from the
// network.
type Subscriber interface {
	Subscribe() (Subscription, error)
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
	Broadcast(ctx context.Context, header *ExtendedHeader) error
}

// Exchange encompasses the behavior necessary to request ExtendedHeaders
// from the network.
type Exchange interface {
	// Start starts Exchange.
	Start(context.Context) error
	// Stop stops Exchange.
	Stop(context.Context) error
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

	// ErrNoHead is returned when Store does not contain Head of the chain,
	ErrNoHead = fmt.Errorf("header/store: no chain head")
)

// Store encompasses the behavior necessary to store and retrieve ExtendedHeaders
// from a node's local storage.
type Store interface {
	// Open opens and initializes Store.
	Open(context.Context) error

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
	// It requires them to be adjacent and in ascending order.
	Append(context.Context, ...*ExtendedHeader) error
}
