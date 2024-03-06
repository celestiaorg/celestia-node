package file

import (
	"context"
	"io"

	logging "github.com/ipfs/go-log/v2"

	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/share"
)

var log = logging.Logger("store/file")

// TODO: add validation of input parameters for Share, AxisHalf and Data methods.
type EdsFile interface {
	io.Closer
	// Reader returns binary reader for the file.
	Reader() (io.Reader, error)
	// Size returns square size of the file.
	Size() int
	// DataHash returns data hash of the file.
	DataHash() share.DataHash
	// Share returns share and corresponding proof for the given axis and share index in this axis.
	Share(ctx context.Context, x, y int) (*share.ShareWithProof, error)
	// AxisHalf returns shares for the first half of the axis of the given type and index.
	AxisHalf(ctx context.Context, axisType rsmt2d.Axis, axisIdx int) ([]share.Share, error)
	// Data returns data for the given namespace and row index.
	Data(ctx context.Context, namespace share.Namespace, rowIdx int) (share.NamespacedRow, error)
	// EDS returns extended data square stored in the file.
	EDS(ctx context.Context) (*rsmt2d.ExtendedDataSquare, error)
}
