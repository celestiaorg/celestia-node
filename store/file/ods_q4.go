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

var _ eds.AccessorStreamer = (*ODSQ4)(nil)

// ODSQ4 is an Accessor that combines ODS and Q4 files.
// It extends the ODS with the ability to read Q4 of the EDS.
// Reading from the fourth quadrant allows to serve samples from Q2 and Q3 quadrants of the square,
// without reading entire Q1.
type ODSQ4 struct {
	Ods *ODS
	Q4  *Q4
}

// CreateODSQ4 creates ODS and Q4 files under the given FS paths.
func CreateODSQ4(
	pathODS, pathQ4 string,
	roots *share.AxisRoots,
	eds *rsmt2d.ExtendedDataSquare,
) error {
	if err := CreateODS(pathODS, roots, eds); err != nil {
		return fmt.Errorf("failed to create ODS file: %w", err)
	}

	if err := CreateQ4(pathQ4, roots, eds); err != nil {
		return fmt.Errorf("failed to create Q4 file: %w", err)
	}

	return nil
}

// OpenODSQ4 open ODS and Q4 files under the given FS path
// and combines them into ODSQ4.
func OpenODSQ4(pathODS, pathQ4 string) (*ODSQ4, error) {
	ods, err := OpenODS(pathODS)
	if err != nil {
		return nil, fmt.Errorf("failed to open ODS file: %w", err)
	}

	q4, err := OpenQ4(pathQ4)
	if err != nil {
		return nil, fmt.Errorf("failed to open Q4 file: %w", err)
	}

	return &ODSQ4{Ods: ods, Q4: q4}, nil
}

func (odsq4 *ODSQ4) Size(ctx context.Context) int {
	return odsq4.Ods.Size(ctx)
}

func (odsq4 *ODSQ4) DataHash(ctx context.Context) (share.DataHash, error) {
	return odsq4.Ods.DataHash(ctx)
}

func (odsq4 *ODSQ4) AxisRoots(ctx context.Context) (*share.AxisRoots, error) {
	return odsq4.Ods.AxisRoots(ctx)
}

func (odsq4 *ODSQ4) Sample(ctx context.Context, rowIdx, colIdx int) (shwap.Sample, error) {
	// use native AxisHalf implementation, to read axis from Q4 quadrant when possible
	half, err := odsq4.AxisHalf(ctx, rsmt2d.Row, rowIdx)
	if err != nil {
		return shwap.Sample{}, fmt.Errorf("reading axis: %w", err)
	}
	shares, err := half.Extended()
	if err != nil {
		return shwap.Sample{}, fmt.Errorf("extending shares: %w", err)
	}
	return shwap.SampleFromShares(shares, rsmt2d.Row, rowIdx, colIdx)
}

func (odsq4 *ODSQ4) AxisHalf(ctx context.Context, axisType rsmt2d.Axis, axisIdx int) (eds.AxisHalf, error) {
	size := odsq4.Size(ctx) // TODO(@Wondertan): Should return error.
	if axisIdx < size/2 {
		half, err := odsq4.Ods.AxisHalf(ctx, axisType, axisIdx)
		if err != nil {
			return eds.AxisHalf{}, fmt.Errorf("reading axis half: %w", err)
		}
		return half, nil
	}

	return odsq4.Q4.AxisHalf(ctx, axisType, axisIdx)
}

func (odsq4 *ODSQ4) RowNamespaceData(ctx context.Context,
	namespace share.Namespace,
	rowIdx int,
) (shwap.RowNamespaceData, error) {
	half, err := odsq4.AxisHalf(ctx, rsmt2d.Row, rowIdx)
	if err != nil {
		return shwap.RowNamespaceData{}, fmt.Errorf("reading axis: %w", err)
	}
	shares, err := half.Extended()
	if err != nil {
		return shwap.RowNamespaceData{}, fmt.Errorf("extending shares: %w", err)
	}
	return shwap.RowNamespaceDataFromShares(shares, namespace, rowIdx)
}

func (odsq4 *ODSQ4) Shares(ctx context.Context) ([]share.Share, error) {
	return odsq4.Ods.Shares(ctx)
}

func (odsq4 *ODSQ4) Reader() (io.Reader, error) {
	return odsq4.Ods.Reader()
}

func (odsq4 *ODSQ4) Close() error {
	return errors.Join(
		odsq4.Ods.Close(),
		odsq4.Q4.Close(),
	)
}
