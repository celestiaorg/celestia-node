package share

import (
	"context"

	"github.com/celestiaorg/nmt/namespace"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/service"
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
	// SharesAvailable subjectively validates if Shares committed to the given Root are available on the Network.
	SharesAvailable(context.Context, *share.Root) error
	// ProbabilityOfAvailability calculates the probability of the data square
	// being available based on the number of samples collected.
	ProbabilityOfAvailability(context.Context) float64
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
		SharesAvailable           func(context.Context, *share.Root) error `perm:"public"`
		ProbabilityOfAvailability func(context.Context) float64            `perm:"public"`
		GetShare                  func(
			ctx context.Context,
			dah *share.Root,
			row, col int,
		) (share.Share, error) `perm:"public"`
		GetShares func(
			ctx context.Context,
			root *share.Root,
		) ([][]share.Share, error) `perm:"public"`
		GetSharesByNamespace func(
			ctx context.Context,
			root *share.Root,
			namespace namespace.ID,
		) ([]share.Share, error) `perm:"public"`
	}
}

func (api *API) SharesAvailable(ctx context.Context, root *share.Root) error {
	return api.Internal.SharesAvailable(ctx, root)
}

func (api *API) ProbabilityOfAvailability(ctx context.Context) float64 {
	return api.Internal.ProbabilityOfAvailability(ctx)
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

type module struct {
	shareService *service.ShareService
}

func (m *module) SharesAvailable(ctx context.Context, root *share.Root) error {
	return m.shareService.SharesAvailable(ctx, root)
}

func (m *module) ProbabilityOfAvailability(ctx context.Context) float64 {
	return m.shareService.ProbabilityOfAvailability(ctx)
}

func (m *module) GetShare(ctx context.Context, dah *share.Root, row, col int) (share.Share, error) {
	return m.shareService.GetShare(ctx, dah, row, col)
}
func (m *module) GetShares(ctx context.Context, root *share.Root) ([][]share.Share, error) {
	return m.shareService.GetShares(ctx, root)
}

func (m *module) GetSharesByNamespace(
	ctx context.Context,
	root *share.Root,
	namespace namespace.ID,
) ([]share.Share, error) {
	return m.shareService.GetSharesByNamespace(ctx, root, namespace)
}
