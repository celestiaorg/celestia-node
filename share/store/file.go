package store

import (
	"context"
	"io"

	"github.com/celestiaorg/nmt"
	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/share"
)

type File interface {
	io.Closer
	Size() int
	ShareWithProof(ctx context.Context, axisType rsmt2d.Axis, axisIdx, shrIdx int) (share.Share, nmt.Proof, error)
	Axis(ctx context.Context, axisType rsmt2d.Axis, axisIdx int) ([]share.Share, error)
	AxisHalf(ctx context.Context, axisType rsmt2d.Axis, axisIdx int) ([]share.Share, error)
	Data(ctx context.Context, namespace share.Namespace, axisIdx int) ([]share.NamespacedRow, error)
	EDS(ctx context.Context) (*rsmt2d.ExtendedDataSquare, error)
}
