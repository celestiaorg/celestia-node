package header

import (
	"context"

	libhead "github.com/celestiaorg/go-header"
	"github.com/celestiaorg/go-header/sync"

	"github.com/celestiaorg/celestia-node/header"
)

// Module exposes the functionality needed for querying headers from the network.
// Any method signature changed here needs to also be changed in the API struct.
//
//go:generate mockgen -destination=mocks/api.go -package=mocks . Module
type Module interface {
	// LocalHead returns the ExtendedHeader of the chain head.
	LocalHead(context.Context) (*header.ExtendedHeader, error)

	// GetByHash returns the header of the given hash from the node's header store.
	GetByHash(ctx context.Context, hash libhead.Hash) (*header.ExtendedHeader, error)
	// GetRangeByHeight returns the given range (from:to) of ExtendedHeaders
	// from the node's header store and verifies that the returned headers are
	// adjacent to each other.
	GetRangeByHeight(
		ctx context.Context,
		from *header.ExtendedHeader,
		to uint64,
	) ([]*header.ExtendedHeader, error)
	// GetByHeight returns the ExtendedHeader at the given height if it is
	// currently available.
	GetByHeight(context.Context, uint64) (*header.ExtendedHeader, error)
	// WaitForHeight blocks until the header at the given height has been processed
	// by the store or context deadline is exceeded.
	WaitForHeight(context.Context, uint64) (*header.ExtendedHeader, error)

	// SyncState returns the current state of the header Syncer.
	SyncState(context.Context) (sync.State, error)
	// SyncWait blocks until the header Syncer is synced to network head.
	SyncWait(ctx context.Context) error
	// NetworkHead provides the Syncer's view of the current network head.
	NetworkHead(ctx context.Context) (*header.ExtendedHeader, error)

	// Subscribe to recent ExtendedHeaders from the network.
	Subscribe(ctx context.Context) (<-chan *header.ExtendedHeader, error)
}

// API is a wrapper around Module for the RPC.
type API struct {
	Internal struct {
		LocalHead func(context.Context) (*header.ExtendedHeader, error) `perm:"read"`
		GetByHash func(
			ctx context.Context,
			hash libhead.Hash,
		) (*header.ExtendedHeader, error) `perm:"read"`
		GetRangeByHeight func(
			context.Context,
			*header.ExtendedHeader,
			uint64,
		) ([]*header.ExtendedHeader, error) `perm:"read"`
		GetByHeight   func(context.Context, uint64) (*header.ExtendedHeader, error)    `perm:"read"`
		WaitForHeight func(context.Context, uint64) (*header.ExtendedHeader, error)    `perm:"read"`
		SyncState     func(ctx context.Context) (sync.State, error)                    `perm:"read"`
		SyncWait      func(ctx context.Context) error                                  `perm:"read"`
		NetworkHead   func(ctx context.Context) (*header.ExtendedHeader, error)        `perm:"read"`
		Subscribe     func(ctx context.Context) (<-chan *header.ExtendedHeader, error) `perm:"read"`
	}
}

func (api *API) GetByHash(ctx context.Context, hash libhead.Hash) (*header.ExtendedHeader, error) {
	return api.Internal.GetByHash(ctx, hash)
}

func (api *API) GetRangeByHeight(
	ctx context.Context,
	from *header.ExtendedHeader,
	to uint64,
) ([]*header.ExtendedHeader, error) {
	return api.Internal.GetRangeByHeight(ctx, from, to)
}

func (api *API) GetByHeight(ctx context.Context, u uint64) (*header.ExtendedHeader, error) {
	return api.Internal.GetByHeight(ctx, u)
}

func (api *API) WaitForHeight(ctx context.Context, u uint64) (*header.ExtendedHeader, error) {
	return api.Internal.WaitForHeight(ctx, u)
}

func (api *API) LocalHead(ctx context.Context) (*header.ExtendedHeader, error) {
	return api.Internal.LocalHead(ctx)
}

func (api *API) SyncState(ctx context.Context) (sync.State, error) {
	return api.Internal.SyncState(ctx)
}

func (api *API) SyncWait(ctx context.Context) error {
	return api.Internal.SyncWait(ctx)
}

func (api *API) NetworkHead(ctx context.Context) (*header.ExtendedHeader, error) {
	return api.Internal.NetworkHead(ctx)
}

func (api *API) Subscribe(ctx context.Context) (<-chan *header.ExtendedHeader, error) {
	return api.Internal.Subscribe(ctx)
}
