package file

import (
	"context"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/celestiaorg/celestia-app/pkg/wrapper"
	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/share"
)

var _ EdsFile = (*OdsFile)(nil)

type OdsFile struct {
	path string
	hdr  *Header
	fl   *os.File

	lock sync.RWMutex
	ods  square
}

// OpenOdsFile opens an existing file. File has to be closed after usage.
func OpenOdsFile(path string) (*OdsFile, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	h, err := ReadHeader(f)
	if err != nil {
		return nil, err
	}

	return &OdsFile{
		path: path,
		hdr:  h,
		fl:   f,
	}, nil
}

func CreateOdsFile(
	path string,
	datahash share.DataHash,
	eds *rsmt2d.ExtendedDataSquare) (*OdsFile, error) {
	f, err := os.Create(path)
	if err != nil {
		return nil, fmt.Errorf("file create: %w", err)
	}

	h := &Header{
		version:    FileV0,
		shareSize:  share.Size, // TODO: rsmt2d should expose this field
		squareSize: uint16(eds.Width()),
		datahash:   datahash,
	}

	err = writeOdsFile(f, h, eds)
	if err != nil {
		return nil, fmt.Errorf("writing ODS file: %w", err)
	}

	// TODO: fill ods field with data from eds
	return &OdsFile{
		path: path,
		fl:   f,
		hdr:  h,
	}, f.Sync()
}

func writeOdsFile(w io.Writer, h *Header, eds *rsmt2d.ExtendedDataSquare) error {
	_, err := h.WriteTo(w)
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

func (f *OdsFile) Size() int {
	return f.hdr.SquareSize()
}

func (f *OdsFile) Close() error {
	if err := f.ods.close(); err != nil {
		return err
	}
	return f.fl.Close()
}

func (f *OdsFile) DataHash() share.DataHash {
	return f.hdr.DataHash()
}

func (f *OdsFile) Reader() (io.Reader, error) {
	err := f.readOds()
	if err != nil {
		return nil, fmt.Errorf("reading ods: %w", err)
	}
	return f.ods.Reader()
}

func (f *OdsFile) AxisHalf(ctx context.Context, axisType rsmt2d.Axis, axisIdx int) (AxisHalf, error) {
	// read axis from file if axis is in the first quadrant
	if axisIdx < f.Size()/2 {
		shares, err := f.readAxisHalf(axisType, axisIdx)
		if err != nil {
			return AxisHalf{}, fmt.Errorf("reading axis half: %w", err)
		}
		return AxisHalf{
			Shares:   shares,
			IsParity: false,
		}, nil
	}

	err := f.readOds()
	if err != nil {
		return AxisHalf{}, err
	}

	shares, err := f.ods.computeAxisHalf(ctx, axisType, axisIdx)
	if err != nil {
		return AxisHalf{}, fmt.Errorf("computing axis half: %w", err)
	}
	return AxisHalf{
		Shares:   shares,
		IsParity: false,
	}, nil
}

func (f *OdsFile) readAxisHalf(axisType rsmt2d.Axis, axisIdx int) ([]share.Share, error) {
	f.lock.RLock()
	ods := f.ods
	f.lock.RUnlock()
	if ods != nil {
		return f.ods.axisHalf(context.Background(), axisType, axisIdx)
	}

	switch axisType {
	case rsmt2d.Col:
		return f.readCol(axisIdx, 0)
	case rsmt2d.Row:
		return f.readRow(axisIdx)
	}
	return nil, fmt.Errorf("unknown axis")
}

func (f *OdsFile) readOds() error {
	f.lock.Lock()
	defer f.lock.Unlock()
	if f.ods != nil {
		return nil
	}

	// reset file pointer to the beginning of the file
	_, err := f.fl.Seek(HeaderSize, io.SeekStart)
	if err != nil {
		return fmt.Errorf("discarding header: %w", err)
	}

	square, err := readSquare(f.fl, share.Size, f.Size())
	if err != nil {
		return fmt.Errorf("reading ods: %w", err)
	}
	f.ods = square
	return nil
}

func (f *OdsFile) readRow(idx int) ([]share.Share, error) {
	shrLn := int(f.hdr.shareSize)
	odsLn := int(f.hdr.squareSize) / 2

	shrs := make([]share.Share, odsLn)

	pos := idx * odsLn
	offset := pos*shrLn + HeaderSize

	axsData := make([]byte, odsLn*shrLn)
	if _, err := f.fl.ReadAt(axsData, int64(offset)); err != nil {
		return nil, err
	}

	for i := range shrs {
		shrs[i] = axsData[i*shrLn : (i+1)*shrLn]
	}
	return shrs, nil
}

func (f *OdsFile) readCol(axisIdx, quadrantIdx int) ([]share.Share, error) {
	shrLn := int(f.hdr.shareSize)
	odsLn := int(f.hdr.squareSize) / 2
	quadrantOffset := quadrantIdx * odsLn * odsLn * shrLn

	shrs := make([]share.Share, odsLn)

	for i := 0; i < odsLn; i++ {
		pos := axisIdx + i*odsLn
		offset := pos*shrLn + HeaderSize + quadrantOffset

		shr := make(share.Share, shrLn)
		if _, err := f.fl.ReadAt(shr, int64(offset)); err != nil {
			return nil, err
		}
		shrs[i] = shr
	}
	return shrs, nil
}

func (f *OdsFile) Share(ctx context.Context, x, y int) (*share.ShareWithProof, error) {
	axisType, axisIdx, shrIdx := rsmt2d.Row, y, x
	// if the share is in the third quadrant, we need to switch axis type to column because it
	// is more efficient to read single column than reading full ods to calculate single row
	if x < f.Size()/2 && y >= f.Size()/2 {
		axisType, axisIdx, shrIdx = rsmt2d.Col, x, y
	}

	axis, err := f.axis(ctx, axisType, axisIdx)
	if err != nil {
		return nil, fmt.Errorf("reading axis: %w", err)
	}

	return shareWithProof(axis, axisType, axisIdx, shrIdx)
}

func (f *OdsFile) Data(ctx context.Context, namespace share.Namespace, rowIdx int) (share.NamespacedRow, error) {
	shares, err := f.axis(ctx, rsmt2d.Row, rowIdx)
	if err != nil {
		return share.NamespacedRow{}, err
	}
	return ndDataFromShares(shares, namespace, rowIdx)
}

func (f *OdsFile) EDS(_ context.Context) (*rsmt2d.ExtendedDataSquare, error) {
	err := f.readOds()
	if err != nil {
		return nil, err
	}

	return f.ods.eds()
}

func shareWithProof(shares []share.Share, axisType rsmt2d.Axis, axisIdx, shrIdx int) (*share.ShareWithProof, error) {
	tree := wrapper.NewErasuredNamespacedMerkleTree(uint64(len(shares)/2), uint(axisIdx))
	for _, shr := range shares {
		err := tree.Push(shr)
		if err != nil {
			return nil, err
		}
	}

	proof, err := tree.ProveRange(shrIdx, shrIdx+1)
	if err != nil {
		return nil, err
	}

	return &share.ShareWithProof{
		Share: shares[shrIdx],
		Proof: &proof,
		Axis:  axisType,
	}, nil
}

func (f *OdsFile) axis(ctx context.Context, axisType rsmt2d.Axis, axisIdx int) ([]share.Share, error) {
	half, err := f.AxisHalf(ctx, axisType, axisIdx)
	if err != nil {
		return nil, err
	}

	return half.Extended()
}
