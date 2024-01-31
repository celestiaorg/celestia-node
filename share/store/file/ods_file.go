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
	height uint64,
	datahash share.DataHash,
	eds *rsmt2d.ExtendedDataSquare) (*OdsFile, error) {
	f, err := os.Create(path)
	if err != nil {
		return nil, fmt.Errorf("file create: %w", err)
	}

	h := &Header{
		version:    FileV0,
		shareSize:  uint16(len(eds.GetCell(0, 0))), // TODO: rsmt2d should expose this field
		squareSize: uint16(eds.Width()),
		height:     height,
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

	for i := uint(0); i < eds.Width()/2; i++ {
		for j := uint(0); j < eds.Width()/2; j++ {
			// TODO: Implemented buffered write through io.CopyBuffer
			shr := eds.GetCell(i, j)
			if _, err := w.Write(shr); err != nil {
				return err
			}
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

func (f *OdsFile) Height() uint64 {
	return f.hdr.Height()
}

func (f *OdsFile) DataHash() share.DataHash {
	return f.hdr.DataHash()
}

func (f *OdsFile) Reader() (io.Reader, error) {
	err := f.readOds()
	if err != nil {
		return nil, fmt.Errorf("reading ods: %w", err)
	}
	return f.ods.Reader(f.hdr)
}

func (f *OdsFile) AxisHalf(ctx context.Context, axisType rsmt2d.Axis, axisIdx int) ([]share.Share, error) {
	// read axis from file if axis is in the first quadrant
	if axisIdx < f.Size()/2 {
		return f.odsAxisHalf(axisType, axisIdx)
	}

	err := f.readOds()
	if err != nil {
		return nil, err
	}

	return f.ods.computeAxisHalf(ctx, axisType, axisIdx)
}

func (f *OdsFile) odsAxisHalf(axisType rsmt2d.Axis, axisIdx int) ([]share.Share, error) {
	f.lock.RLock()
	defer f.lock.RUnlock()
	shrs, err := f.ods.axisHalf(context.Background(), axisType, axisIdx)
	if err == nil {
		return shrs, nil
	}

	switch axisType {
	case rsmt2d.Col:
		return f.readCol(axisIdx)
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

	square, err := readShares(f.hdr, f.fl)
	if err != nil {
		return fmt.Errorf("reading ods: %w", err)
	}
	f.ods = square
	return nil
}

func (f *OdsFile) readRow(idx int) ([]share.Share, error) {
	if idx >= f.Size()/2 {
		return nil, fmt.Errorf("index is out of ods bounds")
	}

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

func (f *OdsFile) readCol(idx int) ([]share.Share, error) {
	if idx >= f.Size()/2 {
		return nil, fmt.Errorf("index is out of ods bounds")
	}

	shrLn := int(f.hdr.shareSize)
	odsLn := int(f.hdr.squareSize) / 2

	shrs := make([]share.Share, odsLn)

	for i := 0; i < odsLn; i++ {
		pos := idx + i*odsLn
		offset := pos*shrLn + HeaderSize

		shr := make(share.Share, shrLn)
		if _, err := f.fl.ReadAt(shr, int64(offset)); err != nil {
			return nil, err
		}
		shrs[i] = shr
	}
	return shrs, nil
}

func (f *OdsFile) axis(ctx context.Context, axisType rsmt2d.Axis, axisIdx int) ([]share.Share, error) {
	original, err := f.AxisHalf(ctx, axisType, axisIdx)
	if err != nil {
		return nil, err
	}

	return extendShares(codec, original)
}

func extendShares(codec Codec, original []share.Share) ([]share.Share, error) {
	sqLen := len(original) * 2
	enc, err := codec.Encoder(sqLen)
	if err != nil {
		return nil, fmt.Errorf("encoder: %w", err)
	}

	shares := make([]share.Share, sqLen)
	copy(shares, original)
	for j := len(original); j < len(shares); j++ {
		shares[j] = make([]byte, len(original[0]))
	}

	err = enc.Encode(shares)
	if err != nil {
		return nil, fmt.Errorf("encoder: %w", err)
	}

	return shares, nil
}

func (f *OdsFile) Share(ctx context.Context, x, y int) (*share.ShareWithProof, error) {
	axisType, axisIdx, shrIdx := rsmt2d.Row, y, x
	if x < f.Size()/2 && y >= f.Size()/2 {
		axisType, axisIdx, shrIdx = rsmt2d.Col, x, y
	}
	shares, err := f.axis(ctx, axisType, axisIdx)
	if err != nil {
		return nil, err
	}

	tree := wrapper.NewErasuredNamespacedMerkleTree(uint64(f.Size()/2), uint(axisIdx))
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

func (f *OdsFile) Data(ctx context.Context, namespace share.Namespace, rowIdx int) (share.NamespacedRow, error) {
	shares, err := f.axis(ctx, rsmt2d.Row, rowIdx)
	if err != nil {
		return share.NamespacedRow{}, err
	}
	return ndDataFromShares(shares, namespace, rowIdx)
}

func (f *OdsFile) EDS(ctx context.Context) (*rsmt2d.ExtendedDataSquare, error) {
	err := f.readOds()
	if err != nil {
		return nil, err
	}

	return f.ods.eds()
}
