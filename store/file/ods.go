package file

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/share"
	eds "github.com/celestiaorg/celestia-node/share/new_eds"
	"github.com/celestiaorg/celestia-node/share/shwap"
)

var _ eds.AccessorStreamer = (*ODS)(nil)

// ODS implements eds.Accessor as an FS file.
// It stores the original data square(ODS), which is the first quadrant of EDS,
// and it's metadata in file's header.
type ODS struct {
	hdr *headerV0
	fl  *os.File

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

// CreateODS creates a new file under given FS path and
// writes the ODS into it out of given EDS.
// It ensures FS is synced after writing finishes.
// It may leave partially written file if any of the writes fail.
func CreateODS(
	path string,
	roots *share.AxisRoots,
	eds *rsmt2d.ExtendedDataSquare,
) error {
	mod := os.O_RDWR | os.O_CREATE | os.O_EXCL // ensure we fail if already exist
	f, err := os.OpenFile(path, mod, filePermissions)
	if err != nil {
		return fmt.Errorf("creating file: %w", err)
	}

	hdr := &headerV0{
		fileVersion: fileV0,
		fileType:    ods,
		shareSize:   share.Size,
		squareSize:  uint16(eds.Width()),
		datahash:    roots.Hash(),
	}

	err = writeODSFile(f, roots, eds, hdr)
	if errClose := f.Close(); errClose != nil {
		err = errors.Join(err, fmt.Errorf("closing created ODS file: %w", errClose))
	}

	return err
}

// writeQ4File full ODS content into OS File.
func writeODSFile(f *os.File, axisRoots *share.AxisRoots, eds *rsmt2d.ExtendedDataSquare, hdr *headerV0) error {
	// buffering gives us ~4x speed up
	buf := bufio.NewWriterSize(f, writeBufferSize)

	if err := writeHeader(f, hdr); err != nil {
		return fmt.Errorf("writing header: %w", err)
	}

	if err := writeAxisRoots(buf, axisRoots); err != nil {
		return fmt.Errorf("writing axis roots: %w", err)
	}

	if err := writeODS(buf, eds); err != nil {
		return fmt.Errorf("writing ODS: %w", err)
	}

	if err := buf.Flush(); err != nil {
		return fmt.Errorf("flushing ODS file: %w", err)
	}

	if err := f.Sync(); err != nil {
		return fmt.Errorf("syncing file: %w", err)
	}

	return nil
}

// writeODS writes the first quadrant(ODS) of the square to the writer. It writes the quadrant in row-major
// order
func writeODS(w io.Writer, eds *rsmt2d.ExtendedDataSquare) error {
	for i := range eds.Width() / 2 {
		for j := range eds.Width() / 2 {
			shr := eds.GetCell(i, j) // TODO: Avoid copying inside GetCell
			_, err := w.Write(shr)
			if err != nil {
				return fmt.Errorf("writing share: %w", err)
			}
		}
	}
	return nil
}

// writeAxisRoots writes RowRoots followed by ColumnRoots.
func writeAxisRoots(w io.Writer, roots *share.AxisRoots) error {
	for _, root := range roots.RowRoots {
		if _, err := w.Write(root); err != nil {
			return fmt.Errorf("writing row roots: %w", err)
		}
	}

	for _, root := range roots.ColumnRoots {
		if _, err := w.Write(root); err != nil {
			return fmt.Errorf("writing columm roots: %w", err)
		}
	}

	return nil
}

// OpenODS opens an existing ODS file under given FS path.
// It only reads the header with metadata. The other content
// of the File is read lazily.
// If file is empty, the ErrEmptyFile is returned.
// File must be closed after usage.
func OpenODS(path string) (*ODS, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	h, err := readHeader(f)
	if err != nil {
		return nil, err
	}

	return &ODS{
		hdr: h,
		fl:  f,
	}, nil
}

// Size returns EDS size stored in file's header.
func (o *ODS) Size(context.Context) int {
	return o.size()
}

func (o *ODS) size() int {
	return int(o.hdr.squareSize)
}

// DataHash returns root hash of Accessor's underlying EDS.
func (o *ODS) DataHash(context.Context) (share.DataHash, error) {
	return o.hdr.datahash, nil
}

// AxisRoots reads AxisRoots stored in the file. AxisRoots are stored after the header and before the
// ODS data.
func (o *ODS) AxisRoots(context.Context) (*share.AxisRoots, error) {
	roots := make([]byte, o.axisRootsSize())
	n, err := o.fl.ReadAt(roots, int64(o.hdr.Size()))
	if err != nil {
		return nil, fmt.Errorf("reading axis roots: %w", err)
	}
	if n != len(roots) {
		return nil, fmt.Errorf("reading axis roots: expected %d bytes, got %d", len(roots), n)
	}
	rowRoots := make([][]byte, o.size())
	colRoots := make([][]byte, o.size())
	for i := 0; i < o.size(); i++ {
		rowRoots[i] = roots[i*share.AxisRootSize : (i+1)*share.AxisRootSize]
		colRoots[i] = roots[(o.size()+i)*share.AxisRootSize : (o.size()+i+1)*share.AxisRootSize]
	}
	axisRoots := &share.AxisRoots{
		RowRoots:    rowRoots,
		ColumnRoots: colRoots,
	}
	return axisRoots, nil
}

// Close closes the file.
func (o *ODS) Close() error {
	return o.fl.Close()
}

// Sample returns share and corresponding proof for row and column indices. Implementation can
// choose which axis to use for proof. Chosen axis for proof should be indicated in the returned
// Sample.
func (o *ODS) Sample(ctx context.Context, rowIdx, colIdx int) (shwap.Sample, error) {
	// Sample proof axis is selected to optimize read performance.
	// - For the first and second quadrants, we read the row axis because it is more efficient to read
	//   single row than reading full ODS to calculate single column
	// - For the third quadrant, we read the column axis because it is more efficient to read single
	//   column than reading full ODS to calculate single row
	// - For the fourth quadrant, it does not matter which axis we read because we need to read full ODS
	//   to calculate the sample
	axisType, axisIdx, shrIdx := rsmt2d.Row, rowIdx, colIdx
	if colIdx < o.size()/2 && rowIdx >= o.size()/2 {
		axisType, axisIdx, shrIdx = rsmt2d.Col, colIdx, rowIdx
	}

	axis, err := o.axis(ctx, axisType, axisIdx)
	if err != nil {
		return shwap.Sample{}, fmt.Errorf("reading axis: %w", err)
	}

	return shwap.SampleFromShares(axis, axisType, axisIdx, shrIdx)
}

// AxisHalf returns half of shares axis of the given type and index. Side is determined by
// implementation. Implementations should indicate the side in the returned AxisHalf.
func (o *ODS) AxisHalf(_ context.Context, axisType rsmt2d.Axis, axisIdx int) (eds.AxisHalf, error) {
	// Read the axis from the file if the axis is a row and from the top half of the square, or if the
	// axis is a column and from the left half of the square.
	if axisIdx < o.size()/2 {
		half, err := o.readAxisHalf(axisType, axisIdx)
		if err != nil {
			return eds.AxisHalf{}, fmt.Errorf("reading axis half: %w", err)
		}
		return half, nil
	}

	// if axis is from the second half of the square, read full ODS and compute the axis half
	ods, err := o.readODS()
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
func (o *ODS) RowNamespaceData(
	ctx context.Context,
	namespace share.Namespace,
	rowIdx int,
) (shwap.RowNamespaceData, error) {
	shares, err := o.axis(ctx, rsmt2d.Row, rowIdx)
	if err != nil {
		return shwap.RowNamespaceData{}, err
	}
	return shwap.RowNamespaceDataFromShares(shares, namespace, rowIdx)
}

// Shares returns data shares extracted from the Accessor.
func (o *ODS) Shares(context.Context) ([]share.Share, error) {
	ods, err := o.readODS()
	if err != nil {
		return nil, err
	}
	return ods.shares()
}

// Reader returns binary reader for the file. It reads the shares from the ODS part of the square
// row by row.
func (o *ODS) Reader() (io.Reader, error) {
	o.lock.RLock()
	ods := o.ods
	o.lock.RUnlock()
	if ods != nil {
		return ods.reader()
	}

	offset := o.sharesOffset()
	total := int64(o.hdr.shareSize) * int64(o.size()*o.size()/4)
	reader := io.NewSectionReader(o.fl, int64(offset), total)
	return reader, nil
}

func (o *ODS) axis(ctx context.Context, axisType rsmt2d.Axis, axisIdx int) ([]share.Share, error) {
	half, err := o.AxisHalf(ctx, axisType, axisIdx)
	if err != nil {
		return nil, err
	}

	axis, err := half.Extended()
	if err != nil {
		return nil, fmt.Errorf("extending axis half: %w", err)
	}

	return axis, nil
}

func (o *ODS) readAxisHalf(axisType rsmt2d.Axis, axisIdx int) (eds.AxisHalf, error) {
	o.lock.RLock()
	ods := o.ods
	o.lock.RUnlock()
	if ods != nil {
		return o.ods.axisHalf(axisType, axisIdx)
	}

	axisHalf, err := readAxisHalf(
		o.fl,
		axisType,
		o.hdr.ShareSize(),
		o.size(),
		o.sharesOffset(),
		axisIdx,
	)
	if err != nil {
		return eds.AxisHalf{}, fmt.Errorf("reading axis half: %w", err)
	}

	return eds.AxisHalf{
		Shares:   axisHalf,
		IsParity: false,
	}, nil
}

func (o *ODS) sharesOffset() int {
	return o.hdr.Size() + o.axisRootsSize()
}

func (o *ODS) axisRootsSize() int {
	// axis roots are stored in two parts: row roots and column roots, each part has size equal to
	// the square size. Thus, the total amount of roots is equal to the square size * 2.
	return share.AxisRootSize * 2 * o.size()
}

func (o *ODS) readODS() (square, error) {
	o.lock.RLock()
	ods := o.ods
	o.lock.RUnlock()
	if ods != nil {
		return ods, nil
	}

	// reset file pointer to the beginning of the file shares data
	offset := o.sharesOffset()
	_, err := o.fl.Seek(int64(offset), io.SeekStart)
	if err != nil {
		return nil, fmt.Errorf("discarding header: %w", err)
	}

	square, err := readSquare(o.fl, share.Size, o.size())
	if err != nil {
		return nil, fmt.Errorf("reading ODS: %w", err)
	}

	if !o.disableCache {
		o.lock.Lock()
		o.ods = square
		o.lock.Unlock()
	}
	return square, nil
}

func readAxisHalf(r io.ReaderAt, axisTp rsmt2d.Axis, shrLn, edsLn, offset, axisIdx int) ([]share.Share, error) {
	switch axisTp {
	case rsmt2d.Row:
		return readRowHalf(r, shrLn, edsLn, offset, axisIdx)
	case rsmt2d.Col:
		return readColHalf(r, shrLn, edsLn, offset, axisIdx)
	default:
		return nil, fmt.Errorf("unknown axis")
	}
}

func readRowHalf(fl io.ReaderAt, shrLn, edsLn, offset, rowIdx int) ([]share.Share, error) {
	odsLn := edsLn / 2
	rowOffset := rowIdx * odsLn * shrLn
	offset = offset + rowOffset

	shares := make([]share.Share, odsLn)
	axsData := make([]byte, odsLn*shrLn)
	if _, err := fl.ReadAt(axsData, int64(offset)); err != nil {
		return nil, err
	}

	for i := range shares {
		shares[i] = axsData[i*shrLn : (i+1)*shrLn]
	}
	return shares, nil
}

func readColHalf(fl io.ReaderAt, shrLn, edsLn, offset, colIdx int) ([]share.Share, error) {
	odsLn := edsLn / 2
	shares := make([]share.Share, odsLn)
	for i := range shares {
		pos := colIdx + i*odsLn
		offset := offset + pos*shrLn

		shr := make(share.Share, shrLn)
		if _, err := fl.ReadAt(shr, int64(offset)); err != nil {
			return nil, err
		}
		shares[i] = shr
	}
	return shares, nil
}
