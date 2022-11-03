package share

import (
	"context"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/nmt/namespace"
)

var _ Module = (*API)(nil)

// Module provides access to any data square or block share on the network.
//
// All Get methods provided on Module follow the following flow:
//  1. Check local storage for the requested Share.
//  2. If exists
//     * Load from disk
//     * Return
//  3. If not
//     * Find provider on the network
//     * Fetch the Share from the provider
//     * Store the Share
//     * Return
//
// Any method signature changed here needs to also be changed in the API struct.
//
//go:generate mockgen -destination=mocks/api.go -package=mocks . Module
type Module interface {
	// SharesAvailable subjectively validates if Shares committed to the given Root are available on
	// the Network.
	SharesAvailable(context.Context, *share.Root) error
	// ProbabilityOfAvailability calculates the probability of the data square
	// being available based on the number of samples collected.
	ProbabilityOfAvailability() float64
	GetShare(ctx context.Context, dah *share.Root, row, col int) (share.Share, error)
	GetShares(ctx context.Context, root *share.Root) ([][]share.Share, error)
	// GetSharesByNamespace iterates over a square's row roots and accumulates the found shares in the
	// given namespace.ID.
	GetSharesByNamespace(ctx context.Context, root *share.Root, namespace namespace.ID) ([]share.Share, error)
}

// API is a wrapper around Module for the RPC.
// TODO(@distractedm1nd): These structs need to be autogenerated.
type API struct {
	Internal struct {
		SharesAvailable           func(context.Context, *share.Root) error `perm:"read"`
		ProbabilityOfAvailability func() float64                           `perm:"read"`
		GetShare                  func(
			ctx context.Context,
			dah *share.Root,
			row, col int,
		) (share.Share, error) `perm:"read"`
		GetShares func(
			ctx context.Context,
			root *share.Root,
		) ([][]share.Share, error) `perm:"read"`
		GetSharesByNamespace func(
			ctx context.Context,
			root *share.Root,
			namespace namespace.ID,
		) ([]share.Share, error) `perm:"read"`
	}
}

func (api *API) SharesAvailable(ctx context.Context, root *share.Root) error {
	return api.Internal.SharesAvailable(ctx, root)
}

func (api *API) ProbabilityOfAvailability() float64 {
	return api.Internal.ProbabilityOfAvailability()
}

func (api *API) GetShare(ctx context.Context, dah *share.Root, row, col int) (share.Share, error) {
	return api.Internal.GetShare(ctx, dah, row, col)
}

func (api *API) GetShares(ctx context.Context, root *share.Root) ([][]share.Share, error) {
	return api.Internal.GetShares(ctx, root)
}

func (api *API) GetSharesByNamespace(
	ctx context.Context,
	root *share.Root,
	namespace namespace.ID,
) ([]share.Share, error) {
	return api.Internal.GetSharesByNamespace(ctx, root, namespace)
}
