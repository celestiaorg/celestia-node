package shwap

import (
	"context"
	"errors"
	"fmt"

	libshare "github.com/celestiaorg/go-square/v2/share"
	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/header"
)

var (
	// ErrOperationNotSupported is used to indicate that the operation is not supported by the
	// implementation of the getter interface.
	ErrOperationNotSupported = errors.New("operation is not supported")
	// ErrNotFound is used to indicate that requested data could not be found.
	ErrNotFound = errors.New("data not found")
	// ErrInvalidID is used to indicate that an ID failed validation.
	ErrInvalidID = errors.New("invalid shwap ID")
	// ErrOutOfBounds is used to indicate that a passed row or column index is out of bounds of the
	// square size.
	ErrOutOfBounds = fmt.Errorf("index out of bounds: %w", ErrInvalidID)
)

// Getter interface provides a set of accessors for shares by the Root.
// Automatically verifies integrity of shares(exceptions possible depending on the implementation).
//
//go:generate mockgen -destination=getters/mock/getter.go -package=mock . Getter
type Getter interface {
	// GetShare gets a Share by coordinates in EDS.
	GetShare(ctx context.Context, header *header.ExtendedHeader, row, col int) (libshare.Share, error)

	// GetEDS gets the full EDS identified by the given extended header.
	GetEDS(context.Context, *header.ExtendedHeader) (*rsmt2d.ExtendedDataSquare, error)

	// GetSharesByNamespace gets all shares from an EDS within the given namespace.
	// Shares are returned in a row-by-row order if the namespace spans multiple rows.
	// Inclusion of returned data could be verified using Verify method on NamespacedShares.
	// If no shares are found for target namespace non-inclusion could be also verified by calling
	// Verify method.
	GetSharesByNamespace(context.Context, *header.ExtendedHeader, libshare.Namespace) (NamespaceData, error)
}
