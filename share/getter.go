package share

import (
	"context"

	"github.com/celestiaorg/nmt/namespace"
)

// Getter interface provides a set of accessors for shares by the Root.
type Getter interface {
	// GetShare gets a Share by coordinates in EDS.
	GetShare(ctx context.Context, root *Root, row, col int) (Share, error)

	// GetShares gets all the shares.
	GetShares(context.Context, *Root) ([][]Share, error)

	// GetSharesByNamespace gets all the shares of the given namespace.
	GetSharesByNamespace(context.Context, *Root, namespace.ID) ([]Share, error)
}
