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

func (f validation) Size(ctx context.Context) (int, error) {
	size := f.size.Load()
	if size != 0 {
		return int(size), nil
	}

	loaded, err := f.Accessor.Size(ctx)
	if err != nil {
		return 0, fmt.Errorf("loading size: %w", err)
	}
	f.size.Store(int32(loaded))
	return loaded, nil
}

func (f validation) Sample(ctx context.Context, idx shwap.SampleCoords) (shwap.Sample, error) {
	size, err := f.Size(ctx)
	if err != nil {
		return shwap.Sample{}, fmt.Errorf("getting size: %w", err)
	}
	_, err = shwap.NewSampleID(1, idx, size)
	if err != nil {
		return shwap.Sample{}, fmt.Errorf("sample validation: %w", err)
	}
	return f.Accessor.Sample(ctx, idx)
}

func (f validation) AxisHalf(ctx context.Context, axisType rsmt2d.Axis, axisIdx int) (shwap.AxisHalf, error) {
	size, err := f.Size(ctx)
	if err != nil {
		return shwap.AxisHalf{}, fmt.Errorf("getting size: %w", err)
	}

	_, err = shwap.NewRowID(1, axisIdx, size)
	if err != nil {
		return shwap.AxisHalf{}, fmt.Errorf("axis half validation: %w", err)
	}
	return f.Accessor.AxisHalf(ctx, axisType, axisIdx)
}

func (f validation) RowNamespaceData(
	ctx context.Context,
	namespace libshare.Namespace,
	rowIdx int,
) (shwap.RowNamespaceData, error) {
	size, err := f.Size(ctx)
	if err != nil {
		return shwap.RowNamespaceData{}, fmt.Errorf("getting size: %w", err)
	}
	_, err = shwap.NewRowNamespaceDataID(1, rowIdx, namespace, size)
	if err != nil {
		return shwap.RowNamespaceData{}, fmt.Errorf("row namespace data validation: %w", err)
	}
	return f.Accessor.RowNamespaceData(ctx, namespace, rowIdx)
}

func (f validation) RangeNamespaceData(
	ctx context.Context,
	from, to int,
) (shwap.RangeNamespaceData, error) {
	if from >= to {
		return shwap.RangeNamespaceData{}, fmt.Errorf(
			"range validation: `from` %d is >= than `to %d", from, to,
		)
	}
	// Size() always returns EDS size
	edsSize, err := f.Size(ctx)
	if err != nil {
		return shwap.RangeNamespaceData{}, fmt.Errorf("getting size: %w", err)
	}
	odsSize := edsSize / 2
	odsSharesAmount := odsSize * odsSize

	if from > odsSharesAmount {
		return shwap.RangeNamespaceData{}, fmt.Errorf(
			"range validation: invalid start index of the range:%d. ODS shares amount %d",
			from, odsSharesAmount,
		)
	}
	if to > odsSharesAmount {
		return shwap.RangeNamespaceData{}, fmt.Errorf(
			"range validation: invalid end coordinates of the range:%d. ODS shares amount %d",
			to, odsSharesAmount,
		)
	}
	return f.Accessor.RangeNamespaceData(ctx, from, to)
}
