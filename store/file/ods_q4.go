package file

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"sync/atomic"

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
	ods *ODS

	pathQ4   string
	q4Mu     sync.Mutex
	q4Opened atomic.Bool
	q4       *Q4
}

// CreateODSQ4 creates ODS and Q4 files under the given FS paths.
func CreateODSQ4(
	pathODS, pathQ4 string,
	roots *share.AxisRoots,
	eds *rsmt2d.ExtendedDataSquare,
) error {
	errCh := make(chan error)
	go func() {
		// doing this async shaves off ~27% of time for 128 ODS
		// for bigger ODSes the discrepancy is even bigger
		err := CreateQ4(pathQ4, roots, eds)
		if err != nil {
			err = fmt.Errorf("сreating Q4 file: %w", err)
		}

		errCh <- err
	}()

	if err := CreateODS(pathODS, roots, eds); err != nil {
		return fmt.Errorf("creating ODS file: %w", err)
	}

	err := <-errCh
	if err != nil {
		return err
	}

	return nil
}

// OpenODSQ4 opens ODS file under the given FS path. The Q4 is opened lazily
// on demand.
func OpenODSQ4(pathODS, pathQ4 string) (*ODSQ4, error) {
	ods, err := OpenODS(pathODS)
	if err != nil {
		return nil, fmt.Errorf("failed to open ODS: %w", err)
	}

	return &ODSQ4{
		ods:    ods,
		pathQ4: pathQ4,
	}, nil
}

func (odsq4 *ODSQ4) tryLoadQ4() *Q4 {
	// If Q4 was attempted to be opened before, return.
	if odsq4.q4Opened.Load() {
		return odsq4.q4
	}

	odsq4.q4Mu.Lock()
	defer odsq4.q4Mu.Unlock()
	// update bool to make sure we try opening only once
	// no matter if the try was successful or failed.
	if opened := odsq4.q4Opened.Swap(true); opened {
		return odsq4.q4
	}

	q4, err := OpenQ4(odsq4.pathQ4)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}

	if err != nil {
		log.Errorf("opening Q4 file %s: %s", odsq4.pathQ4, err)
		return nil
	}

	odsq4.q4 = q4
	return q4
}

func (odsq4 *ODSQ4) Size(ctx context.Context) int {
	return odsq4.ods.Size(ctx)
}

func (odsq4 *ODSQ4) DataHash(ctx context.Context) (share.DataHash, error) {
	return odsq4.ods.DataHash(ctx)
}

func (odsq4 *ODSQ4) AxisRoots(ctx context.Context) (*share.AxisRoots, error) {
	return odsq4.ods.AxisRoots(ctx)
}

func (odsq4 *ODSQ4) Sample(ctx context.Context, rowIdx, colIdx int) (shwap.Sample, error) {
	// use native AxisHalf implementation, to read axis from q4 quadrant when possible
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

	var acsr eds.Accessor = odsq4.ods
	if axisIdx >= size/2 {
		if q4 := odsq4.tryLoadQ4(); q4 != nil {
			acsr = odsq4.q4
		}
	}

	half, err := acsr.AxisHalf(ctx, axisType, axisIdx)
	if err != nil {
		return eds.AxisHalf{}, fmt.Errorf("reading axis half: %w", err)
	}

	return half, nil
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
	return odsq4.ods.Shares(ctx)
}

func (odsq4 *ODSQ4) Reader() (io.Reader, error) {
	return odsq4.ods.Reader()
}

func (odsq4 *ODSQ4) Close() error {
	err := odsq4.ods.Close()
	if err != nil {
		err = fmt.Errorf("closing ODS file: %w", err)
	}

	odsq4.q4Mu.Lock() // wait in case file is being opened
	defer odsq4.q4Mu.Unlock()
	if odsq4.q4Opened.Load() {
		errQ4 := odsq4.q4.Close()
		if errQ4 != nil {
			errQ4 = fmt.Errorf("closing Q4 file: %w", errQ4)
			err = errors.Join(err, errQ4)
		}
	}
	return err
}
