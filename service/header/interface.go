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
	NextHeader(ctx context.Context) (*ExtendedHeader, error)
	Cancel()
}

// Exchange encompasses the behavior necessary to request ExtendedHeaders
// and respond to ExtendedHeader requests from the network.
type Exchange interface {
	RequestHeaders(ctx context.Context, request *ExtendedHeaderRequest) ([]*ExtendedHeader, error)
	RespondToHeadersRequest(ctx context.Context, request *ExtendedHeaderRequest) error
}

// Store encompasses the behavior necessary to store and retrieve ExtendedHeaders
// from a node's local storage.
type Store interface {
	Get(ctx context.Context, hash tmbytes.HexBytes) (*ExtendedHeader, error)
	GetMany(ctx context.Context, hashes []tmbytes.HexBytes) ([]*ExtendedHeader, error)
	GetByHeight(ctx context.Context, height int64) (*ExtendedHeader, error)
	GetRangeByHeight(ctx context.Context, from, to int64) ([]*ExtendedHeader, error)
	Put(ctx context.Context, header *ExtendedHeader) error
	PutMany(ctx context.Context, headers []*ExtendedHeader) error
}
