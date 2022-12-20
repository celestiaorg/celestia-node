package share

import (
	"context"
	"errors"

	"github.com/celestiaorg/celestia-node/share/ipld"
	"github.com/celestiaorg/nmt/namespace"
)

// Getter interface provides a set of accessors for shares by the Root.
// Automatically verifies integrity of shares(exceptions possible depending on the implementation)
type Getter interface {
	// GetShare gets a Share by coordinates in EDS.
	GetShare(ctx context.Context, root *Root, row, col int) (Share, error)

	// GetShares gets all shares in an EDS.
	// Shares are returned in a row-by-row order.
	GetShares(context.Context, *Root) ([][]Share, error)

	// GetSharesByNamespace gets all shares from an EDS within the given namespace.
	// Shares are returned in a row-by-row order if the namespace spans multiple rows.
	GetSharesByNamespace(context.Context, *Root, namespace.ID) (NamespaceShares, error)
}

// NamespaceShares represents all the shares with proofs of a specific namespace of an EDS.
type NamespaceShares []RowNamespaceShares

// RowNamespaceShares represents all the shares with proofs of a specific namespace within a Row of
// an EDS.
type RowNamespaceShares struct {
	Shares []Share
	Proof  *ipld.Proof
}

// Verify checks for the inclusion of the NamespacedShares in the Root and verifies their belonging
// to the given namespace.
func (NamespaceShares) Verify(*Root, namespace.ID) error {
	return errors.New("not implemented")
}

// Flatten returns the concatenated slice of all RowNamespaceShares shares.
func (ns NamespaceShares) Flatten() []Share {
	shares := make([]Share, 0)
	for _, row := range ns {
		shares = append(shares, row.Shares...)
	}
	return shares
}
