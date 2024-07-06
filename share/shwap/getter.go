package shwap

import (
	"context"
	"errors"
	"fmt"

	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/share"
)

var (
	// ErrOperationNotSupported is used to indicate that the operation is not supported by the
	// implementation of the getter interface.
	ErrOperationNotSupported = errors.New("operation is not supported")
	// ErrNotFound is used to indicate that requested data could not be found.
	ErrNotFound = errors.New("data not found")
	// ErrOutOfBounds is used to indicate that a passed row or column index is out of bounds of the
	// square size.
	ErrOutOfBounds = fmt.Errorf("index out of bounds: %w", ErrInvalidShwapID)
)

// Getter interface provides a set of accessors for shares by the Root.
// Automatically verifies integrity of shares(exceptions possible depending on the implementation).
//
//go:generate mockgen -destination=mocks/getter.go -package=mocks . Getter
type Getter interface {
	// GetShare gets a Share by coordinates in EDS.
	GetShare(ctx context.Context, header *header.ExtendedHeader, row, col int) (share.Share, error)

	// GetEDS gets the full EDS identified by the given extended header.
	GetEDS(context.Context, *header.ExtendedHeader) (*rsmt2d.ExtendedDataSquare, error)

	// GetSharesByNamespace gets all shares from an EDS within the given namespace.
	// Shares are returned in a row-by-row order if the namespace spans multiple rows.
	// Inclusion of returned data could be verified using Verify method on NamespacedShares.
	// If no shares are found for target namespace non-inclusion could be also verified by calling
	// Verify method.
	GetSharesByNamespace(context.Context, *header.ExtendedHeader, share.Namespace) (NamespacedData, error)
}
