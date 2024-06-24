package file

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/share"
	eds "github.com/celestiaorg/celestia-node/share/new_eds"
	"github.com/celestiaorg/celestia-node/share/shwap"
)

var _ eds.AccessorCloser = (*Q1Q4File)(nil)

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

func CreateQ1Q4File(path string, datahash share.DataHash, eds *rsmt2d.ExtendedDataSquare,
) (*Q1Q4File, error) {
	ods, err := CreateODSFile(path, datahash, eds)
	if err != nil {
		return nil, err
	}

	err = writeQ4(ods.fl, eds)
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

	shares, err := f.readSharesFromQ4(axisType, axisIdx)
	if err != nil {
		return eds.AxisHalf{}, err
	}
	return eds.AxisHalf{
		Shares:   shares,
		IsParity: true,
	}, nil
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

func (f *Q1Q4File) Close() error {
	return f.ods.Close()
}

func writeQ4(w io.Writer, eds *rsmt2d.ExtendedDataSquare) error {
	odsLn := int(eds.Width()) / 2
	for x := odsLn; x < int(eds.Width()); x++ {
		for y := odsLn; y < int(eds.Width()); y++ {
			_, err := w.Write(eds.GetCell(uint(x), uint(y)))
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (f *Q1Q4File) readSharesFromQ4(axisType rsmt2d.Axis, axisIdx int) ([]share.Share, error) {
	q4idx := axisIdx - f.ods.size()/2
	if q4idx < 0 {
		return nil, errors.New("invalid index requested from Q4")
	}
	switch axisType {
	case rsmt2d.Col:
		return readCol(f.ods.fl, f.ods.hdr, q4idx, 1)
	case rsmt2d.Row:
		return readRow(f.ods.fl, f.ods.hdr, q4idx, 1)
	default:
		return nil, fmt.Errorf("invalid axis type: %d", axisType)
	}
}
