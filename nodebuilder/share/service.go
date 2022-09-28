package share

import (
	"context"

	"github.com/ipfs/go-blockservice"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/nmt/namespace"
)

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
type Module interface {
	share.Availability
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	GetShare(ctx context.Context, dah *share.Root, row, col int) (share.Share, error)
	GetShares(ctx context.Context, root *share.Root) ([][]share.Share, error)
	GetSharesByNamespace(ctx context.Context, root *share.Root, namespace namespace.ID) ([]share.Share, error)
}

func NewModule(bServ blockservice.BlockService, avail share.Availability) Module {
	return share.NewService(bServ, avail)
}
