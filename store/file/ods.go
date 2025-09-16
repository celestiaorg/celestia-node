package file

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"

	libshare "github.com/celestiaorg/go-square/v2/share"
	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds"
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
// It may leave partially written file if any of the writes fail.
func CreateODS(
	path string,
	roots *share.AxisRoots,
	eds *rsmt2d.ExtendedDataSquare,
) error {
	mod := os.O_RDWR | os.O_CREATE | os.O_EXCL // ensure we fail if already exist
	f, err := os.OpenFile(path, mod, filePermissions)
	if err != nil {
		return fmt.Errorf("creating ODS file: %w", err)
	}

	shareSize := len(eds.GetCell(0, 0))
	hdr := &headerV0{
		fileVersion: fileV0,
		shareSize:   uint16(shareSize),
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

	return nil
}

// writeODS writes the first quadrant(ODS) of the square to the writer. It writes the quadrant in
// row-major order. Write finishes once all the shares are written or on the first instance of tail
// padding share. Tail padding share are constant and aren't stored.
func writeODS(w io.Writer, eds *rsmt2d.ExtendedDataSquare) error {
	for i := range eds.Width() / 2 {
		for j := range eds.Width() / 2 {
			shr := eds.GetCell(i, j) // TODO: Avoid copying inside GetCell
			ns, err := libshare.NewNamespaceFromBytes(shr[:libshare.NamespaceSize])
			if err != nil {
				return fmt.Errorf("creating namespace: %w", err)
			}
			if ns.Equals(libshare.TailPaddingNamespace) {
				return nil
			}

			_, err = w.Write(shr)
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
			return fmt.Errorf("writing column roots: %w", err)
		}
	}

	return nil
}

// ValidateODSSize checks if the file under given FS path has the expected size.
func ValidateODSSize(path string, eds *rsmt2d.ExtendedDataSquare) error {
	ods, err := OpenODS(path)
	if err != nil {
		return fmt.Errorf("opening file: %w", err)
	}

	shares, err := filledSharesAmount(eds)
	if err != nil {
		return fmt.Errorf("calculating shares amount: %w", err)
	}
	shareSize := len(eds.GetCell(0, 0))
	expectedSize := ods.hdr.OffsetWithRoots() + shares*shareSize

	info, err := ods.fl.Stat()
	if err != nil {
		return fmt.Errorf("getting file info: %w", err)
	}
	if info.Size() != int64(expectedSize) {
		return fmt.Errorf("file size mismatch: expected %d, got %d", expectedSize, info.Size())
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
func (o *ODS) Size(context.Context) (int, error) {
	return o.size(), nil
}

func (o *ODS) size() int {
	return int(o.hdr.squareSize)
}

// DataHash returns root hash of Accessor's underlying EDS.
func (o *ODS) DataHash(context.Context) (share.DataHash, error) {
	return o.hdr.datahash, nil
}

// AxisRoots reads AxisRoots stored in the file. AxisRoots are stored after the header and before
// the ODS data.
func (o *ODS) AxisRoots(context.Context) (*share.AxisRoots, error) {
	roots := make([]byte, o.hdr.RootsSize())
	n, err := o.fl.ReadAt(roots, int64(o.hdr.Size()))
	if err != nil {
		return nil, fmt.Errorf("reading axis roots: %w", err)
	}
	if n != len(roots) {
		return nil, fmt.Errorf("reading axis roots: expected %d bytes, got %d", len(roots), n)
	}
	rowRoots := make([][]byte, o.size())
	colRoots := make([][]byte, o.size())
	for i := range o.size() {
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
func (o *ODS) Sample(ctx context.Context, idx shwap.SampleCoords) (shwap.Sample, error) {
	// Sample proof axis is selected to optimize read performance.
	// - For the first and second quadrants, we read the row axis because it is more efficient to read
	//   single row than reading full ODS to calculate single column
	// - For the third quadrant, we read the column axis because it is more efficient to read single
	//   column than reading full ODS to calculate single row
	// - For the fourth quadrant, it does not matter which axis we read because we need to read full ODS
	//   to calculate the sample
	rowIdx, colIdx := idx.Row, idx.Col

	size, err := o.Size(ctx)
	if err != nil {
		return shwap.Sample{}, fmt.Errorf("getting size: %w", err)
	}

	axisType, axisIdx, shrIdx := rsmt2d.Row, rowIdx, colIdx
	if colIdx < size/2 && rowIdx >= size/2 {
		axisType, axisIdx, shrIdx = rsmt2d.Col, colIdx, rowIdx
	}

	axis, err := o.axis(ctx, axisType, axisIdx)
	if err != nil {
		return shwap.Sample{}, fmt.Errorf("reading axis: %w", err)
	}

	idxNew := shwap.SampleCoords{Row: axisIdx, Col: shrIdx}

	return shwap.SampleFromShares(axis, axisType, idxNew)
}

// AxisHalf returns half of shares axis of the given type and index. Side is determined by
// implementation. Implementations should indicate the side in the returned AxisHalf.
func (o *ODS) AxisHalf(_ context.Context, axisType rsmt2d.Axis, axisIdx int) (shwap.AxisHalf, error) {
	// Read the axis from the file if the axis is a row and from the top half of the square, or if the
	// axis is a column and from the left half of the square.
	if axisIdx < o.size()/2 {
		half, err := o.readAxisHalf(axisType, axisIdx)
		if err != nil {
			return shwap.AxisHalf{}, fmt.Errorf("reading axis half: %w", err)
		}
		return half, nil
	}

	// if axis is from the second half of the square, read full ODS and compute the axis half
	ods, err := o.readODS()
	if err != nil {
		return shwap.AxisHalf{}, err
	}

	half, err := ods.computeAxisHalf(axisType, axisIdx)
	if err != nil {
		return shwap.AxisHalf{}, fmt.Errorf("computing axis half: %w", err)
	}
	return half, nil
}

// RowNamespaceData returns data for the given namespace and row index.
func (o *ODS) RowNamespaceData(
	ctx context.Context,
	namespace libshare.Namespace,
	rowIdx int,
) (shwap.RowNamespaceData, error) {
	shares, err := o.axis(ctx, rsmt2d.Row, rowIdx)
	if err != nil {
		return shwap.RowNamespaceData{}, err
	}
	return shwap.RowNamespaceDataFromShares(shares, namespace, rowIdx)
}

// Shares returns data shares extracted from the Accessor.
func (o *ODS) Shares(context.Context) ([]libshare.Share, error) {
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

	offset := o.hdr.OffsetWithRoots()
	total := int64(o.hdr.shareSize) * int64(o.size()*o.size()/4)
	reader := io.NewSectionReader(o.fl, int64(offset), total)
	return reader, nil
}

func (o *ODS) RangeNamespaceData(
	ctx context.Context,
	from, to int,
) (shwap.RangeNamespaceData, error) {
	size, err := o.Size(ctx)
	if err != nil {
		return shwap.RangeNamespaceData{}, err
	}
	odsSize := size / 2
	fromCoords, err := shwap.SampleCoordsFrom1DIndex(from, odsSize)
	if err != nil {
		return shwap.RangeNamespaceData{}, err
	}
	// to is an exclusive index.
	toCoords, err := shwap.SampleCoordsFrom1DIndex(to-1, odsSize)
	if err != nil {
		return shwap.RangeNamespaceData{}, err
	}

	shares := make([][]libshare.Share, toCoords.Row-fromCoords.Row+1)
	for row, idx := fromCoords.Row, 0; row <= toCoords.Row; row++ {
		half, err := o.readAxisHalf(rsmt2d.Row, row)
		if err != nil {
			return shwap.RangeNamespaceData{}, fmt.Errorf("reading axis half: %w", err)
		}

		sh, err := half.Extended()
		if err != nil {
			return shwap.RangeNamespaceData{}, fmt.Errorf("extending the data: %w", err)
		}
		shares[idx] = sh
		idx++
	}
	return shwap.RangeNamespaceDataFromShares(shares, fromCoords, toCoords)
}

func (o *ODS) axis(ctx context.Context, axisType rsmt2d.Axis, axisIdx int) ([]libshare.Share, error) {
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

func (o *ODS) readAxisHalf(axisType rsmt2d.Axis, axisIdx int) (shwap.AxisHalf, error) {
	o.lock.RLock()
	ods := o.ods
	o.lock.RUnlock()
	if ods != nil {
		return o.ods.axisHalf(axisType, axisIdx)
	}

	axisHalf, err := readAxisHalf(o.fl, axisType, axisIdx, o.hdr, o.hdr.OffsetWithRoots())
	if err != nil {
		return shwap.AxisHalf{}, fmt.Errorf("reading axis half: %w", err)
	}

	return shwap.AxisHalf{
		Shares:   axisHalf,
		IsParity: false,
	}, nil
}

func (o *ODS) readODS() (square, error) {
	if !o.disableCache {
		o.lock.RLock()
		ods := o.ods
		o.lock.RUnlock()
		if ods != nil {
			return ods, nil
		}

		// not cached, read and cache
		o.lock.Lock()
		defer o.lock.Unlock()
	}

	offset := o.hdr.OffsetWithRoots()
	shareSize := o.hdr.ShareSize()
	odsBytes := o.hdr.SquareSize() / 2
	odsSizeInBytes := shareSize * odsBytes * odsBytes
	reader := io.NewSectionReader(o.fl, int64(offset), int64(odsSizeInBytes))
	ods, err := readSquare(reader, shareSize, o.size())
	if err != nil {
		return nil, fmt.Errorf("reading ODS: %w", err)
	}

	if !o.disableCache {
		o.ods = ods
	}
	return ods, nil
}

func readAxisHalf(r io.ReaderAt, axisTp rsmt2d.Axis, axisIdx int, hdr *headerV0, offset int) ([]libshare.Share, error) {
	switch axisTp {
	case rsmt2d.Row:
		return readRowHalf(r, axisIdx, hdr, offset)
	case rsmt2d.Col:
		return readColHalf(r, axisIdx, hdr, offset)
	default:
		return nil, fmt.Errorf("unknown axis")
	}
}

// readRowHalf reads specific Row half from the file in a single IO operation.
// If some or all shares are missing, tail padding shares are returned instead.
func readRowHalf(r io.ReaderAt, rowIdx int, hdr *headerV0, offset int) ([]libshare.Share, error) {
	odsLn := hdr.SquareSize() / 2
	rowOffset := rowIdx * odsLn * hdr.ShareSize()
	offset += rowOffset

	shares := make([]libshare.Share, odsLn)
	axsData := make([]byte, odsLn*hdr.ShareSize())
	n, err := r.ReadAt(axsData, int64(offset))
	if err != nil && !errors.Is(err, io.EOF) {
		// unknown error
		return nil, err
	}

	shrsRead := n / hdr.ShareSize()
	for i := range shares {
		if i > shrsRead-1 {
			// partial or empty row was read
			// fill the rest with tail padding it
			shares[i] = libshare.TailPaddingShare()
			continue
		}
		sh, err := libshare.NewShare(axsData[i*hdr.ShareSize() : (i+1)*hdr.ShareSize()])
		if err != nil {
			return nil, err
		}
		shares[i] = *sh
	}
	return shares, nil
}

// readColHalf reads specific Col half from the file in a single IO operation.
// If some or all shares are missing, tail padding shares are returned instead.
func readColHalf(r io.ReaderAt, colIdx int, hdr *headerV0, offset int) ([]libshare.Share, error) {
	odsLn := hdr.SquareSize() / 2
	shares := make([]libshare.Share, odsLn)
	for i := range shares {
		pos := colIdx + i*odsLn
		offset := offset + pos*hdr.ShareSize()

		shr := make([]byte, hdr.ShareSize())
		n, err := r.ReadAt(shr, int64(offset))
		if err != nil && !errors.Is(err, io.EOF) {
			// unknown error
			return nil, err
		}
		if n == 0 {
			// no shares left
			// fill the rest with tail padding
			for ; i < len(shares); i++ {
				shares[i] = libshare.TailPaddingShare()
			}
			return shares, nil
		}

		sh, err := libshare.NewShare(shr)
		if err != nil {
			return nil, err
		}
		// we got a share
		shares[i] = *sh
	}
	return shares, nil
}

// filledSharesAmount returns the amount of shares in the ODS that are not tail padding.
func filledSharesAmount(eds *rsmt2d.ExtendedDataSquare) (int, error) {
	var amount int
	for i := range eds.Width() / 2 {
		for j := range eds.Width() / 2 {
			rawShr := eds.GetCell(i, j)
			shr, err := libshare.NewShare(rawShr)
			if err != nil {
				return 0, fmt.Errorf("creating share at(%d,%d): %w", i, j, err)
			}
			if shr.Namespace().Equals(libshare.TailPaddingNamespace) {
				break
			}
			amount++
		}
	}
	return amount, nil
}
