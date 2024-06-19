package file

import (
	"context"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/share"
	eds "github.com/celestiaorg/celestia-node/share/new_eds"
	"github.com/celestiaorg/celestia-node/share/shwap"
)

var _ eds.AccessorCloser = (*ODSFile)(nil)

type ODSFile struct {
	path string
	hdr  *headerV0
	fl   *os.File

	lock sync.RWMutex
	// ods stores an in-memory cache of the original data square to enhance read performance. This
	// cache is particularly beneficial for operations that require reading the entire square, such as:
	// - Serving samples from the fourth quadrant of the square, which necessitates reconstructing data
	// from all rows. - Streaming the entire ODS by Reader(), ensuring efficient data delivery without
	// repeated file reads. - Serving full ODS data by Shares().
	// Storing the square in memory allows for efficient single-read operations, avoiding the need for
	// piecemeal reads by rows or columns, and facilitates quick access to data for these operations.
	ods square
}

// OpenODSFile opens an existing file. File has to be closed after usage.
func OpenODSFile(path string) (*ODSFile, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	h, err := readHeader(f)
	if err != nil {
		return nil, err
	}

	return &ODSFile{
		path: path,
		hdr:  h,
		fl:   f,
	}, nil
}

// CreateODSFile creates a new file. File has to be closed after usage.
func CreateODSFile(
	path string,
	datahash share.DataHash,
	eds *rsmt2d.ExtendedDataSquare,
) (*ODSFile, error) {
	f, err := os.Create(path)
	if err != nil {
		return nil, fmt.Errorf("file create: %w", err)
	}

	h := &headerV0{
		fileVersion: fileV0,
		fileType:    ODS,
		shareSize:   share.Size,
		squareSize:  uint16(eds.Width()),
		datahash:    datahash,
	}

	err = writeODSFile(f, h, eds)
	if err != nil {
		return nil, fmt.Errorf("writing ODS file: %w", err)
	}

	err = f.Sync()
	if err != nil {
		return nil, fmt.Errorf("syncing file: %w", err)
	}

	return &ODSFile{
		path: path,
		fl:   f,
		hdr:  h,
	}, nil
}

func writeODSFile(w io.Writer, h *headerV0, eds *rsmt2d.ExtendedDataSquare) error {
	err := writeHeader(w, h)
	if err != nil {
		return err
	}

	for _, shr := range eds.FlattenedODS() {
		if _, err := w.Write(shr); err != nil {
			return err
		}
	}
	return nil
}

// Size returns square size of the Accessor.
func (f *ODSFile) Size(context.Context) int {
	return f.size()
}

func (f *ODSFile) size() int {
	return int(f.hdr.squareSize)
}

// Close closes the file.
func (f *ODSFile) Close() error {
	return f.fl.Close()
}

// Sample returns share and corresponding proof for row and column indices. Implementation can
// choose which axis to use for proof. Chosen axis for proof should be indicated in the returned
// Sample.
func (f *ODSFile) Sample(ctx context.Context, rowIdx, colIdx int) (shwap.Sample, error) {
	// Sample proof axis is selected to optimize read performance.
	// - For the first and second quadrants, we read the row axis because it is more efficient to read
	//   single row than reading full ODS to calculate single column
	// - For the third quadrant, we read the column axis because it is more efficient to read single
	//   column than reading full ODS to calculate single row
	// - For the fourth quadrant, it does not matter which axis we read because we need to read full ODS
	//   to calculate the sample
	axisType, axisIdx, shrIdx := rsmt2d.Row, rowIdx, colIdx
	if colIdx < f.size()/2 && rowIdx >= f.size()/2 {
		axisType, axisIdx, shrIdx = rsmt2d.Col, colIdx, rowIdx
	}

	axis, err := f.axis(ctx, axisType, axisIdx)
	if err != nil {
		return shwap.Sample{}, fmt.Errorf("reading axis: %w", err)
	}

	return shwap.SampleFromShares(axis, axisType, axisIdx, shrIdx)
}

// AxisHalf returns half of shares axis of the given type and index. Side is determined by
// implementation. Implementations should indicate the side in the returned AxisHalf.
func (f *ODSFile) AxisHalf(_ context.Context, axisType rsmt2d.Axis, axisIdx int) (eds.AxisHalf, error) {
	// Read the axis from the file if the axis is a row and from the top half of the square, or if the
	// axis is a column and from the left half of the square.
	if axisIdx < f.size()/2 {
		half, err := f.readAxisHalf(axisType, axisIdx)
		if err != nil {
			return eds.AxisHalf{}, fmt.Errorf("reading axis half: %w", err)
		}
		return half, nil
	}

	// if axis is from the second half of the square, read full ODS and compute the axis half
	err := f.readODS()
	if err != nil {
		return eds.AxisHalf{}, err
	}

	half, err := f.ods.computeAxisHalf(axisType, axisIdx)
	if err != nil {
		return eds.AxisHalf{}, fmt.Errorf("computing axis half: %w", err)
	}
	return half, nil
}

// RowNamespaceData returns data for the given namespace and row index.
func (f *ODSFile) RowNamespaceData(
	ctx context.Context,
	namespace share.Namespace,
	rowIdx int,
) (shwap.RowNamespaceData, error) {
	shares, err := f.axis(ctx, rsmt2d.Row, rowIdx)
	if err != nil {
		return shwap.RowNamespaceData{}, err
	}
	return shwap.RowNamespaceDataFromShares(shares, namespace, rowIdx)
}

// Shares returns data shares extracted from the Accessor.
func (f *ODSFile) Shares(context.Context) ([]share.Share, error) {
	err := f.readODS()
	if err != nil {
		return nil, err
	}
	return f.ods.shares()
}

func (f *ODSFile) readAxisHalf(axisType rsmt2d.Axis, axisIdx int) (eds.AxisHalf, error) {
	f.lock.RLock()
	ODS := f.ods
	f.lock.RUnlock()
	if ODS != nil {
		return f.ods.axisHalf(axisType, axisIdx)
	}

	switch axisType {
	case rsmt2d.Col:
		col, err := f.readCol(axisIdx, 0)
		return eds.AxisHalf{
			Shares:   col,
			IsParity: false,
		}, err
	case rsmt2d.Row:
		row, err := f.readRow(axisIdx)
		return eds.AxisHalf{
			Shares:   row,
			IsParity: false,
		}, err
	}
	return eds.AxisHalf{}, fmt.Errorf("unknown axis")
}

func (f *ODSFile) readODS() error {
	f.lock.Lock()
	defer f.lock.Unlock()
	if f.ods != nil {
		return nil
	}

	// reset file pointer to the beginning of the file shares data
	_, err := f.fl.Seek(int64(f.hdr.Size()), io.SeekStart)
	if err != nil {
		return fmt.Errorf("discarding header: %w", err)
	}

	square, err := readSquare(f.fl, share.Size, f.size())
	if err != nil {
		return fmt.Errorf("reading ODS: %w", err)
	}
	f.ods = square
	return nil
}

func (f *ODSFile) readRow(idx int) ([]share.Share, error) {
	shrLn := int(f.hdr.shareSize)
	odsLn := f.size() / 2

	shares := make([]share.Share, odsLn)

	pos := idx * odsLn
	offset := f.hdr.Size() + pos*shrLn

	axsData := make([]byte, odsLn*shrLn)
	if _, err := f.fl.ReadAt(axsData, int64(offset)); err != nil {
		return nil, err
	}

	for i := range shares {
		shares[i] = axsData[i*shrLn : (i+1)*shrLn]
	}
	return shares, nil
}

func (f *ODSFile) readCol(axisIdx, quadrantIdx int) ([]share.Share, error) {
	shrLn := int(f.hdr.shareSize)
	odsLn := f.size() / 2
	quadrantOffset := quadrantIdx * odsLn * odsLn * shrLn

	shares := memPools.get(odsLn).getHalfAxis()
	for i := range shares {
		pos := axisIdx + i*odsLn
		offset := f.hdr.Size() + quadrantOffset + pos*shrLn

		if _, err := f.fl.ReadAt(shares[i], int64(offset)); err != nil {
			return nil, err
		}
	}
	return shares, nil
}

func (f *ODSFile) axis(ctx context.Context, axisType rsmt2d.Axis, axisIdx int) ([]share.Share, error) {
	half, err := f.AxisHalf(ctx, axisType, axisIdx)
	if err != nil {
		return nil, err
	}

	return half.Extended()
}
