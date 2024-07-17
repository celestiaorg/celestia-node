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

var _ eds.AccessorStreamer = (*ODSFile)(nil)

var ErrEmptyFile = fmt.Errorf("file is empty")

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
	// disableCache is a flag that, when set to true, disables the in-memory cache of the original data
	// Used for testing and benchmarking purposes, this flag allows for the evaluation of the
	// performance.
	disableCache bool
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
		fileType:    ods,
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

// DataRoot returns root hash of Accessor's underlying EDS.
func (f *ODSFile) DataRoot(context.Context) (share.DataHash, error) {
	return f.hdr.datahash, nil
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
	ods, err := f.readODS()
	if err != nil {
		return eds.AxisHalf{}, err
	}

	half, err := ods.computeAxisHalf(axisType, axisIdx)
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
	ods, err := f.readODS()
	if err != nil {
		return nil, err
	}
	return ods.shares()
}

// Reader returns binary reader for the file. It reads the shares from the ODS part of the square
// row by row.
func (f *ODSFile) Reader() (io.Reader, error) {
	f.lock.RLock()
	ods := f.ods
	f.lock.RUnlock()
	if ods != nil {
		return ods.reader()
	}

	offset := int64(f.hdr.Size())
	total := int64(f.hdr.shareSize) * int64(f.size()*f.size()/4)
	reader := io.NewSectionReader(f.fl, offset, total)
	return reader, nil
}

func (f *ODSFile) readAxisHalf(axisType rsmt2d.Axis, axisIdx int) (eds.AxisHalf, error) {
	f.lock.RLock()
	ods := f.ods
	f.lock.RUnlock()
	if ods != nil {
		return f.ods.axisHalf(axisType, axisIdx)
	}

	switch axisType {
	case rsmt2d.Col:
		col, err := readCol(f.fl, f.hdr, axisIdx, 0)
		return eds.AxisHalf{
			Shares:   col,
			IsParity: false,
		}, err
	case rsmt2d.Row:
		row, err := readRow(f.fl, f.hdr, axisIdx, 0)
		return eds.AxisHalf{
			Shares:   row,
			IsParity: false,
		}, err
	}
	return eds.AxisHalf{}, fmt.Errorf("unknown axis")
}

func (f *ODSFile) readODS() (square, error) {
	f.lock.RLock()
	ods := f.ods
	f.lock.RUnlock()
	if ods != nil {
		return ods, nil
	}

	// reset file pointer to the beginning of the file shares data
	_, err := f.fl.Seek(int64(f.hdr.Size()), io.SeekStart)
	if err != nil {
		return nil, fmt.Errorf("discarding header: %w", err)
	}

	square, err := readSquare(f.fl, share.Size, f.size())
	if err != nil {
		return nil, fmt.Errorf("reading ODS: %w", err)
	}

	if !f.disableCache {
		f.lock.Lock()
		f.ods = square
		f.lock.Unlock()
	}
	return square, nil
}

func readRow(fl io.ReaderAt, hdr *headerV0, rowIdx, quadrantIdx int) ([]share.Share, error) {
	shrLn := int(hdr.shareSize)
	odsLn := int(hdr.squareSize / 2)
	quadrantOffset := quadrantIdx * odsLn * odsLn * shrLn

	shares := make([]share.Share, odsLn)

	pos := rowIdx * odsLn
	offset := hdr.Size() + quadrantOffset + pos*shrLn

	axsData := make([]byte, odsLn*shrLn)
	if _, err := fl.ReadAt(axsData, int64(offset)); err != nil {
		return nil, err
	}

	for i := range shares {
		shares[i] = axsData[i*shrLn : (i+1)*shrLn]
	}
	return shares, nil
}

func readCol(fl io.ReaderAt, hdr *headerV0, colIdx, quadrantIdx int) ([]share.Share, error) {
	shrLn := int(hdr.shareSize)
	odsLn := int(hdr.squareSize / 2)
	quadrantOffset := quadrantIdx * odsLn * odsLn * shrLn

	shares := make([]share.Share, odsLn)
	for i := range shares {
		pos := colIdx + i*odsLn
		offset := hdr.Size() + quadrantOffset + pos*shrLn

		shr := make(share.Share, shrLn)
		if _, err := fl.ReadAt(shr, int64(offset)); err != nil {
			return nil, err
		}
		shares[i] = shr
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
