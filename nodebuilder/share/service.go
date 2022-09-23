package share

import (
	"context"

	"github.com/ipfs/go-blockservice"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/nmt/namespace"
)

// Service provides access to any data square or block share on the network.
//
// All Get methods provided on Service follow the following flow:
//  1. Check local storage for the requested Share.
//  2. If exists
//     * Load from disk
//     * Return
//  3. If not
//     * Find provider on the network
//     * Fetch the Share from the provider
//     * Store the Share
//     * Return
type Service interface {
	share.Availability
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	GetShare(ctx context.Context, dah *share.Root, row, col int) (share.Share, error)
	GetShares(ctx context.Context, root *share.Root) ([][]share.Share, error)
	GetSharesByNamespace(ctx context.Context, root *share.Root, namespace namespace.ID) ([]share.Share, error)
}

func NewService(bServ blockservice.BlockService, avail share.Availability) Service {
	return share.NewService(bServ, avail)
}
