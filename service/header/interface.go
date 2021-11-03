package header

import (
	"context"

	tmbytes "github.com/celestiaorg/celestia-core/libs/bytes"
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

// Exchange encompasses the behavior necessary to request ExtendedHeaders
// from the network.
type Exchange interface {
	// RequestHeader performs a request for the ExtendedHeader at the given
	// height to the network. Note that the ExtendedHeader must be verified
	// thereafter.
	RequestHeader(ctx context.Context, height int64) (*ExtendedHeader, error)
	// RequestHeaders performs a request for the given range of ExtendedHeaders
	// to the network. Note that the ExtendedHeaders must be verified thereafter.
	RequestHeaders(ctx context.Context, from, to int64) ([]*ExtendedHeader, error)
}

// Store encompasses the behavior necessary to store and retrieve ExtendedHeaders
// from a node's local storage.
type Store interface {
	// Head returns the ExtendedHeader of the chain head.
	Head() (*ExtendedHeader, error)

	// Get returns the ExtendedHeader corresponding to the given hash.
	Get(ctx context.Context, hash tmbytes.HexBytes) (*ExtendedHeader, error)
	// GetMany returns the ExtendedHeaders corresponding to the given hashes.
	GetMany(ctx context.Context, hashes []tmbytes.HexBytes) ([]*ExtendedHeader, error)

	// GetByHeight returns the ExtendedHeader corresponding to the given block height.
	GetByHeight(ctx context.Context, height int64) (*ExtendedHeader, error)
	// GetRangeByHeight returns the given range of ExtendedHeaders.
	GetRangeByHeight(ctx context.Context, from, to int64) ([]*ExtendedHeader, error)

	// Put stores the given ExtendedHeader.
	Put(ctx context.Context, header *ExtendedHeader) error
	// PutMany stores the given ExtendedHeaders.
	PutMany(ctx context.Context, headers []*ExtendedHeader) error
}
