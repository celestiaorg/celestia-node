package eds

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"

	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/shwap"
)

var _ Accessor = validation{}

// ErrOutOfBounds is returned whenever an index is out of bounds.
var ErrOutOfBounds = errors.New("index is out of bounds")

// validation is a  Accessor implementation that performs sanity checks on methods. It wraps
// another  Accessor and performs bounds checks on index arguments.
type validation struct {
	Accessor
	size atomic.Int32
}

func WithValidation(f Accessor) Accessor {
	return &validation{Accessor: f}
}

func (f validation) Size(ctx context.Context) int {
	size := f.size.Load()
	if size == 0 {
		loaded := f.Accessor.Size(ctx)
		f.size.Store(int32(loaded))
		return loaded
	}
	return int(size)
}

func (f validation) Sample(ctx context.Context, rowIdx, colIdx int) (shwap.Sample, error) {
	if err := validateIndexBounds(ctx, f, colIdx); err != nil {
		return shwap.Sample{}, fmt.Errorf("col: %w", err)
	}
	if err := validateIndexBounds(ctx, f, rowIdx); err != nil {
		return shwap.Sample{}, fmt.Errorf("row: %w", err)
	}
	return f.Accessor.Sample(ctx, rowIdx, colIdx)
}

func (f validation) AxisHalf(ctx context.Context, axisType rsmt2d.Axis, axisIdx int) (AxisHalf, error) {
	if err := validateIndexBounds(ctx, f, axisIdx); err != nil {
		return AxisHalf{}, fmt.Errorf("%s: %w", axisType, err)
	}
	return f.Accessor.AxisHalf(ctx, axisType, axisIdx)
}

func (f validation) RowNamespaceData(
	ctx context.Context,
	namespace share.Namespace,
	rowIdx int,
) (shwap.RowNamespaceData, error) {
	if err := validateIndexBounds(ctx, f, rowIdx); err != nil {
		return shwap.RowNamespaceData{}, fmt.Errorf("row: %w", err)
	}
	return f.Accessor.RowNamespaceData(ctx, namespace, rowIdx)
}

// validateIndexBounds checks if the index is within the bounds of the eds.
func validateIndexBounds(ctx context.Context, f Accessor, idx int) error {
	size := f.Size(ctx)
	if idx < 0 || idx >= size {
		return fmt.Errorf("%w: index %d is out of bounds: [0, %d)", ErrOutOfBounds, idx, size)
	}
	return nil
}
