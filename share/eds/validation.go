package eds

import (
	"context"
	"fmt"
	"sync/atomic"

	libshare "github.com/celestiaorg/go-square/v2/share"
	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/share/shwap"
)

var _ Accessor = validation{}

// validation is a  Accessor implementation that performs sanity checks on methods. It wraps
// another  Accessor and performs bounds checks on index arguments.
type validation struct {
	Accessor
	size *atomic.Int32
}

func WithValidation(f Accessor) Accessor {
	return &validation{Accessor: f, size: new(atomic.Int32)}
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

func (f validation) Sample(ctx context.Context, idx shwap.SampleCoords) (shwap.Sample, error) {
	_, err := shwap.NewSampleID(1, idx, f.Size(ctx))
	if err != nil {
		return shwap.Sample{}, fmt.Errorf("sample validation: %w", err)
	}
	return f.Accessor.Sample(ctx, idx)
}

func (f validation) AxisHalf(ctx context.Context, axisType rsmt2d.Axis, axisIdx int) (AxisHalf, error) {
	_, err := shwap.NewRowID(1, axisIdx, f.Size(ctx))
	if err != nil {
		return AxisHalf{}, fmt.Errorf("axis half validation: %w", err)
	}
	return f.Accessor.AxisHalf(ctx, axisType, axisIdx)
}

func (f validation) RowNamespaceData(
	ctx context.Context,
	namespace libshare.Namespace,
	rowIdx int,
) (shwap.RowNamespaceData, error) {
	_, err := shwap.NewRowNamespaceDataID(1, rowIdx, namespace, f.Size(ctx))
	if err != nil {
		return shwap.RowNamespaceData{}, fmt.Errorf("row namespace data validation: %w", err)
	}
	return f.Accessor.RowNamespaceData(ctx, namespace, rowIdx)
}

func (f validation) RangeNamespaceData(
	ctx context.Context,
	ns libshare.Namespace,
	from, to shwap.SampleCoords,
) (shwap.RangeNamespaceData, error) {
	if from.Row > to.Row {
		return shwap.RangeNamespaceData{}, fmt.Errorf(
			"range validation: from row %d is > then to row %d", from.Row, to.Row,
		)
	}
	odsSharesAmount := f.Size(ctx)/2 - 1
	if from.Row > odsSharesAmount || from.Col > odsSharesAmount {
		return shwap.RangeNamespaceData{}, fmt.Errorf(
			"range validation: invalid start coordinates of the range:{%d;%d}. ODS shares amount %d",
			from.Row, from.Col, odsSharesAmount,
		)
	}
	if to.Row > odsSharesAmount || to.Col > odsSharesAmount {
		return shwap.RangeNamespaceData{}, fmt.Errorf(
			"range validation: invalid end coordinates of the range:{%d;%d}. ODS shares amount %d",
			to.Row, to.Col, odsSharesAmount,
		)
	}
	return f.Accessor.RangeNamespaceData(ctx, ns, from, to)
}
