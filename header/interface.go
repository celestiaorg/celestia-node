package header

import (
	"context"
	"errors"
	"fmt"

	"github.com/ipfs/go-blockservice"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	tmbytes "github.com/tendermint/tendermint/libs/bytes"
	core "github.com/tendermint/tendermint/types"
)

// ConstructFn aliases a function that creates an ExtendedHeader.
type ConstructFn = func(
	context.Context,
	*core.Block,
	*core.Commit,
	*core.ValidatorSet,
	blockservice.BlockService,
) (*ExtendedHeader, error)

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
	// Stop removes header-sub validator and closes the topic.
	Stop(context.Context) error
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
	Getter
}

var (
	// ErrNotFound is returned when there is no requested header.
	ErrNotFound = errors.New("header: not found")

	// ErrNoHead is returned when Store is empty (does not contain any known header).
	ErrNoHead = fmt.Errorf("header/store: no chain head")

	// ErrHeadersLimitExceeded is returned when ExchangeServer receives header request for more
	// than maxRequestSize headers.
	ErrHeadersLimitExceeded = errors.New("header/p2p: header limit per 1 request exceeded")
)

// ErrNonAdjacent is returned when Store is appended with a header not adjacent to the stored head.
type ErrNonAdjacent struct {
	Head      int64
	Attempted int64
}

func (ena *ErrNonAdjacent) Error() string {
	return fmt.Sprintf("header/store: non-adjacent: head %d, attempted %d", ena.Head, ena.Attempted)
}

// Store encompasses the behavior necessary to store and retrieve ExtendedHeaders
// from a node's local storage.
type Store interface {
	// Start starts the store.
	Start(context.Context) error

	// Stop stops the store by preventing further writes
	// and waiting till the ongoing ones are done.
	Stop(context.Context) error

	// Getter encompasses all getter methods for headers.
	Getter

	// Init initializes Store with the given head, meaning it is initialized with the genesis header.
	Init(context.Context, *ExtendedHeader) error

	// Height reports current height of the chain head.
	Height() uint64

	// Has checks whether ExtendedHeader is already stored.
	Has(context.Context, tmbytes.HexBytes) (bool, error)

	// Append stores and verifies the given ExtendedHeader(s).
	// It requires them to be adjacent and in ascending order,
	// as it applies them contiguously on top of the current head height.
	// It returns the amount of successfully applied headers,
	// so caller can understand what given header was invalid, if any.
	Append(context.Context, ...*ExtendedHeader) (int, error)
}

// Getter contains the behavior necessary for a component to retrieve
// headers that have been processed during header sync.
type Getter interface {
	Head

	// Get returns the ExtendedHeader corresponding to the given hash.
	Get(context.Context, tmbytes.HexBytes) (*ExtendedHeader, error)

	// GetByHeight returns the ExtendedHeader corresponding to the given block height.
	GetByHeight(context.Context, uint64) (*ExtendedHeader, error)

	// GetRangeByHeight returns the given range of ExtendedHeaders.
	GetRangeByHeight(ctx context.Context, from, amount uint64) ([]*ExtendedHeader, error)

	// GetVerifiedRange requests the header range from the provided ExtendedHeader and
	// verifies that the returned headers are adjacent to each other.
	GetVerifiedRange(ctx context.Context, from *ExtendedHeader, amount uint64) ([]*ExtendedHeader, error)
}

// Head contains the behavior necessary for a component to retrieve
// the chain head. Note that "chain head" is subjective to the component
// reporting it.
type Head interface {
	// Head returns the latest known header.
	Head(context.Context) (*ExtendedHeader, error)
}
