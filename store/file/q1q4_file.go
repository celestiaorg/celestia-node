package file

import (
	"context"
	"fmt"
	"io"

	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/share"
	eds "github.com/celestiaorg/celestia-node/share/new_eds"
	"github.com/celestiaorg/celestia-node/share/shwap"
)

var _ eds.AccessorStreamer = (*Q1Q4File)(nil)

// Q1Q4File represents a file that contains the first and fourth quadrants of an extended data
// square. It extends the ODSFile with the ability to read the fourth quadrant of the square.
// Reading from the fourth quadrant allows to serve samples from Q2 and Q4 quadrants of the square,
// without the need to read entire Q1.
type Q1Q4File struct {
	ods *ODSFile
}

func OpenQ1Q4File(path string) (*Q1Q4File, error) {
	ods, err := OpenODSFile(path)
	if err != nil {
		return nil, err
	}

	return &Q1Q4File{
		ods: ods,
	}, nil
}

func CreateQ1Q4File(path string, roots *share.AxisRoots, eds *rsmt2d.ExtendedDataSquare) (*Q1Q4File, error) {
	ods, err := CreateODSFile(path, roots, eds)
	if err != nil {
		return nil, err
	}

	err = writeQuadrant(ods.fl, eds, int(ods.hdr.shareSize), 3)
	if err != nil {
		return nil, fmt.Errorf("writing Q4: %w", err)
	}

	return &Q1Q4File{
		ods: ods,
	}, nil
}

func (f *Q1Q4File) Size(ctx context.Context) int {
	return f.ods.Size(ctx)
}

func (f *Q1Q4File) DataHash(ctx context.Context) (share.DataHash, error) {
	return f.ods.DataHash(ctx)
}

func (f *Q1Q4File) AxisRoots(ctx context.Context) (*share.AxisRoots, error) {
	return f.ods.AxisRoots(ctx)
}

func (f *Q1Q4File) Sample(ctx context.Context, rowIdx, colIdx int) (shwap.Sample, error) {
	// use native AxisHalf implementation, to read axis from Q4 quandrant when possible
	half, err := f.AxisHalf(ctx, rsmt2d.Row, rowIdx)
	if err != nil {
		return shwap.Sample{}, fmt.Errorf("reading axis: %w", err)
	}
	shares, err := half.Extended()
	if err != nil {
		return shwap.Sample{}, fmt.Errorf("extending shares: %w", err)
	}
	return shwap.SampleFromShares(shares, rsmt2d.Row, rowIdx, colIdx)
}

func (f *Q1Q4File) AxisHalf(_ context.Context, axisType rsmt2d.Axis, axisIdx int) (eds.AxisHalf, error) {
	if axisIdx < f.ods.size()/2 {
		half, err := f.ods.readAxisHalf(axisType, axisIdx)
		if err != nil {
			return eds.AxisHalf{}, fmt.Errorf("reading axis half: %w", err)
		}
		return half, nil
	}

	return f.readAxisHalfFromQ4(axisType, axisIdx)
}

func (f *Q1Q4File) RowNamespaceData(ctx context.Context,
	namespace share.Namespace,
	rowIdx int,
) (shwap.RowNamespaceData, error) {
	half, err := f.AxisHalf(ctx, rsmt2d.Row, rowIdx)
	if err != nil {
		return shwap.RowNamespaceData{}, fmt.Errorf("reading axis: %w", err)
	}
	shares, err := half.Extended()
	if err != nil {
		return shwap.RowNamespaceData{}, fmt.Errorf("extending shares: %w", err)
	}
	return shwap.RowNamespaceDataFromShares(shares, namespace, rowIdx)
}

func (f *Q1Q4File) Shares(ctx context.Context) ([]share.Share, error) {
	return f.ods.Shares(ctx)
}

func (f *Q1Q4File) Reader() (io.Reader, error) {
	return f.ods.Reader()
}

func (f *Q1Q4File) Close() error {
	return f.ods.Close()
}

func (f *Q1Q4File) readAxisHalfFromQ4(axisType rsmt2d.Axis, axisIdx int) (eds.AxisHalf, error) {
	q4idx := axisIdx - f.ods.size()/2
	if q4idx < 0 {
		return eds.AxisHalf{}, fmt.Errorf("invalid axis index for Q4: %d", axisIdx)
	}
	offset := f.ods.sharesOffset()
	switch axisType {
	case rsmt2d.Col:
		shares, err := readCol(f.ods.fl, f.ods.hdr, offset, 1, q4idx)
		if err != nil {
			return eds.AxisHalf{}, err
		}
		return eds.AxisHalf{
			Shares:   shares,
			IsParity: true,
		}, nil
	case rsmt2d.Row:
		shares, err := readRow(f.ods.fl, f.ods.hdr, offset, 1, q4idx)
		if err != nil {
			return eds.AxisHalf{}, err
		}
		return eds.AxisHalf{
			Shares:   shares,
			IsParity: true,
		}, nil
	default:
		return eds.AxisHalf{}, fmt.Errorf("invalid axis type: %d", axisType)
	}
}
