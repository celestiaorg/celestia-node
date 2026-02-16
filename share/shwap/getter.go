package shwap

import (
	"context"
	"errors"
	"fmt"

	libshare "github.com/celestiaorg/go-square/v3/share"
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
	// ErrNoSampleIndicies is used to indicate that no indicies where given to process.
	ErrNoSampleIndicies = errors.New("no sample indicies to fetch")
)

// Getter interface provides a set of accessors for shares by the Root.
// Automatically verifies integrity of shares(exceptions possible depending on the implementation).
//
//go:generate mockgen -destination=getters/mock/getter.go -package=mock . Getter
type Getter interface {
	// GetSamples retrieves multiple shares from the Extended Data Square (EDS) at the specified
	// coordinates. The coordinates are provided as a slice of SampleCoords, and the method returns
	// a slice of samples in the same order as the requested coordinates. If some samples cannot be
	// found, the corresponding positions in the returned slice will be empty, but the method will
	// still return the available samples without error.
	GetSamples(
		ctx context.Context,
		header *header.ExtendedHeader,
		indices []SampleCoords,
	) ([]Sample, error)

	// GetEDS retrieves the complete Extended Data Square (EDS) identified by the given header.
	// The EDS contains all shares organized in a 2D matrix format with erasure coding.
	// This method is useful when full access to all shares in the square is required.
	GetEDS(
		ctx context.Context,
		header *header.ExtendedHeader,
	) (*rsmt2d.ExtendedDataSquare, error)

	// GetRow retrieves all shares from a specific row in the Extended Data Square (EDS).
	// The row is identified by its index (rowIdx) and the header. This method is useful
	// for operations that need to process all shares in a particular row of the EDS.
	GetRow(
		ctx context.Context,
		header *header.ExtendedHeader,
		rowIdx int,
	) (Row, error)

	// GetNamespaceData retrieves all shares that belong to the specified namespace within
	// the Extended Data Square (EDS). The shares are returned in a row-by-row order,
	// maintaining the original layout if the namespace spans multiple rows. The returned
	// data can be verified for inclusion using the Verify method on NamespacedShares.
	// If no shares are found for the target namespace, non-inclusion can be verified
	// using the same Verify method.
	GetNamespaceData(
		ctx context.Context,
		header *header.ExtendedHeader,
		namespace libshare.Namespace,
	) (NamespaceData, error)

	// GetRangeNamespaceData retrieves a range of shares within a specific namespace in the
	// Extended Data Square (EDS). The range is defined by from and to indexes, which
	// specify the start and end points of the desired range, where `from` is inclusive
	// and `to` is exclusive indexes. The shares are returned in a row-by-row order
	// if the range spans multiple rows.
	GetRangeNamespaceData(
		ctx context.Context,
		header *header.ExtendedHeader,
		from, to int,
	) (RangeNamespaceData, error)

	// GetBlob retrieves a single blob identified by its namespace and commitment
	// from the Extended Data Square (EDS) referenced by the given header.
	GetBlob(_ context.Context, _ *header.ExtendedHeader, _ libshare.Namespace, commitment []byte) (*Blob, error)

	// GetBlobs retrieves all blobs belonging to the given namespace from the
	// Extended Data Square (EDS) referenced by the given header.
	GetBlobs(_ context.Context, _ *header.ExtendedHeader, _ libshare.Namespace) ([]*Blob, error)
}
