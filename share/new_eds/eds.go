package eds

import (
	"context"
	"io"

	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/shwap"
)

// EDS is an interface for accessing extended data square data.
type EDS interface {
	io.Closer
	// Size returns square size of the file.
	Size() int
	// Sample returns share and corresponding proof for row and column indices. Implementation can
	// choose which axis to use for proof. Chosen axis for proof should be indicated in the returned
	// Sample.
	Sample(ctx context.Context, rowIdx, colIdx int) (shwap.Sample, error)
	// AxisHalf returns half of shares axis of the given type and index. Side is determined by
	// implementation. Implementations should indicate the side in the returned AxisHalf.
	AxisHalf(ctx context.Context, axisType rsmt2d.Axis, axisIdx int) (AxisHalf, error)
	// RowData returns data for the given namespace and row index.
	RowData(ctx context.Context, namespace share.Namespace, rowIdx int) (shwap.RowNamespaceData, error)
	// EDS returns extended data square stored in the file.
	EDS(ctx context.Context) (*rsmt2d.ExtendedDataSquare, error)
}
