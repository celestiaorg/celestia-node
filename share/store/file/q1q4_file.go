package file

import (
	"context"
	"fmt"
	"io"

	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/share"
)

var _ EdsFile = (*Q1Q4File)(nil)

type Q1Q4File struct {
	*OdsFile
}

func OpenQ1Q4File(path string) (*Q1Q4File, error) {
	ods, err := OpenOdsFile(path)
	if err != nil {
		return nil, err
	}

	return &Q1Q4File{
		OdsFile: ods,
	}, nil
}

func CreateQ1Q4File(
	path string,
	datahash share.DataHash,
	eds *rsmt2d.ExtendedDataSquare) (*Q1Q4File, error) {
	ods, err := CreateOdsFile(path, datahash, eds)
	if err != nil {
		return nil, err
	}

	err = writeQ4(ods.fl, eds)
	if err != nil {
		return nil, fmt.Errorf("writing Q4: %w", err)
	}

	return &Q1Q4File{
		OdsFile: ods,
	}, nil

}

func (f *Q1Q4File) AxisHalf(_ context.Context, axisType rsmt2d.Axis, axisIdx int) (AxisHalf, error) {
	if axisIdx < f.Size()/2 {
		half, err := f.OdsFile.readAxisHalf(axisType, axisIdx)
		if err != nil {
			return AxisHalf{}, fmt.Errorf("reading axis half: %w", err)
		}
		return AxisHalf{
			Shares:   half,
			IsParity: false,
		}, nil
	}

	var half []share.Share
	var err error
	switch axisType {
	case rsmt2d.Col:
		half, err = f.readCol(axisIdx-f.Size()/2, 1)
	case rsmt2d.Row:
		half, err = f.readRow(axisIdx)
	}
	if err != nil {
		return AxisHalf{}, fmt.Errorf("reading axis: %w", err)
	}
	return AxisHalf{
		Shares:   half,
		IsParity: true,
	}, nil
}

func (f *Q1Q4File) Share(ctx context.Context, x, y int) (*share.ShareWithProof, error) {
	half, err := f.AxisHalf(ctx, rsmt2d.Row, y)
	if err != nil {
		return nil, fmt.Errorf("reading axis: %w", err)
	}
	shares, err := half.Extended()
	if err != nil {
		return nil, fmt.Errorf("extending shares: %w", err)
	}
	return shareWithProof(shares, rsmt2d.Row, y, x)
}

func (f *Q1Q4File) Data(ctx context.Context, namespace share.Namespace, rowIdx int) (share.NamespacedRow, error) {
	half, err := f.AxisHalf(ctx, rsmt2d.Row, rowIdx)
	if err != nil {
		return share.NamespacedRow{}, fmt.Errorf("reading axis: %w", err)
	}
	shares, err := half.Extended()
	if err != nil {
		return share.NamespacedRow{}, fmt.Errorf("extending shares: %w", err)
	}
	return ndDataFromShares(shares, namespace, rowIdx)
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
