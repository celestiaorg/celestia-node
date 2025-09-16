package eds

import (
	"context"
	"io"

	libshare "github.com/celestiaorg/go-square/v2/share"
	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/shwap"
)

// EmptyAccessor is an accessor of an empty EDS block.
var EmptyAccessor = &Rsmt2D{ExtendedDataSquare: share.EmptyEDS()}

// Accessor is an interface for accessing extended data square data.
type Accessor interface {
	// Size returns square size of the Accessor.
	Size(ctx context.Context) (int, error)
	// DataHash returns data hash of the Accessor.
	DataHash(ctx context.Context) (share.DataHash, error)
	// AxisRoots returns share.AxisRoots (DataAvailabilityHeader) of the Accessor.
	AxisRoots(ctx context.Context) (*share.AxisRoots, error)
	// Sample returns share and corresponding proof for row and column indices. Implementation can
	// choose which axis to use for proof. Chosen axis for proof should be indicated in the returned
	// Sample.
	Sample(ctx context.Context, idx shwap.SampleCoords) (shwap.Sample, error)
	// AxisHalf returns half of shares axis of the given type and index. Side is determined by
	// implementation. Implementations should indicate the side in the returned AxisHalf.
	AxisHalf(ctx context.Context, axisType rsmt2d.Axis, axisIdx int) (shwap.AxisHalf, error)
	// RowNamespaceData returns data for the given namespace and row index.
	RowNamespaceData(ctx context.Context, namespace libshare.Namespace, rowIdx int) (shwap.RowNamespaceData, error)
	// Shares returns data (ODS) shares extracted from the Accessor.
	Shares(ctx context.Context) ([]libshare.Share, error)
	// RangeNamespaceData returns data(ODS) shares along with their proofs from the requested range
	// from the Accessor.
	RangeNamespaceData(
		ctx context.Context,
		from, to int,
	) (shwap.RangeNamespaceData, error)
}

// AccessorStreamer is an interface that groups Accessor and Streamer interfaces.
type AccessorStreamer interface {
	Accessor
	Streamer
}

type Streamer interface {
	// Reader returns binary reader for the shares. It should read the shares from the
	// ODS part of the square row by row.
	Reader() (io.Reader, error)
	io.Closer
}

type accessorStreamer struct {
	Accessor
	Streamer
}

func AccessorAndStreamer(a Accessor, s Streamer) AccessorStreamer {
	return &accessorStreamer{a, s}
}
