package store

import (
	"context"
	"io"

	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/share"
)

type EdsFile interface {
	io.Closer
	// Size returns square size of the file.
	Size() int
	// Share returns share and corresponding proof for the given axis and share index in this axis.
	Share(ctx context.Context, x, y int, proofType ProofType) (*share.ShareWithProof, error)
	// AxisHalf returns shares for the first half of the axis of the given type and index.
	AxisHalf(ctx context.Context, axisType rsmt2d.Axis, axisIdx int) ([]share.Share, error)
	// Data returns data for the given namespace and row index.
	Data(ctx context.Context, namespace share.Namespace, rowIdx int) (share.NamespacedRow, error)
	// EDS returns extended data square stored in the file.
	EDS(ctx context.Context) (*rsmt2d.ExtendedDataSquare, error)
}

// ProofType represents type of proof that should be computed for the share.
type ProofType int

const (
	// ProofTypeAny indicates that any proof could be computed for the share.
	ProofTypeAny ProofType = iota
	// ProofTypeRow indicates that only row inclusion proof should be computed for the share.
	ProofTypeRow
	// ProofTypeColumn indicates that only column inclusion proof should be computed for the share.
	ProofTypeColumn
)
