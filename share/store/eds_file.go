package store

import (
	"context"
	"fmt"
	"os"

	"golang.org/x/exp/mmap"

	"github.com/celestiaorg/celestia-app/pkg/wrapper"
	"github.com/celestiaorg/nmt"
	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/share"
)

var _ File = (*EdsFile)(nil)

type EdsFile struct {
	path string
	hdr  *Header
	fl   fileBackend
}

// OpenEdsFile opens an existing file. File has to be closed after usage.
func OpenEdsFile(path string) (*EdsFile, error) {
	f, err := mmap.Open(path)
	if err != nil {
		return nil, err
	}

	h, err := ReadHeader(f)
	if err != nil {
		return nil, err
	}

	// TODO(WWondertan): Validate header
	return &EdsFile{
		path: path,
		hdr:  h,
		fl:   f,
	}, nil
}

func CreateEdsFile(path string, eds *rsmt2d.ExtendedDataSquare) (*EdsFile, error) {
	f, err := os.Create(path)
	if err != nil {
		return nil, err
	}

	h := &Header{
		shareSize:  uint16(len(eds.GetCell(0, 0))), // TODO: rsmt2d should expose this field
		squareSize: uint16(eds.Width()),
		version:    FileV0,
	}

	if _, err = h.WriteTo(f); err != nil {
		return nil, err
	}

	for i := uint(0); i < eds.Width(); i++ {
		for j := uint(0); j < eds.Width(); j++ {
			// TODO: Implemented buffered write through io.CopyBuffer
			shr := eds.GetCell(i, j)
			if _, err := f.Write(shr); err != nil {
				return nil, err
			}
		}
	}

	return &EdsFile{
		path: path,
		fl:   f,
		hdr:  h,
	}, f.Sync()
}

func (f *EdsFile) Size() int {
	return f.hdr.SquareSize()
}

func (f *EdsFile) Close() error {
	return f.fl.Close()
}

func (f *EdsFile) Header() *Header {
	return f.hdr
}

func (f *EdsFile) AxisHalf(_ context.Context, axisType rsmt2d.Axis, axisIdx int) ([]share.Share, error) {
	axis, err := f.axis(axisType, axisIdx)
	if err != nil {
		return nil, err
	}
	return axis[:f.Size()/2], nil
}

func (f *EdsFile) axis(axisType rsmt2d.Axis, axisIdx int) ([]share.Share, error) {
	switch axisType {
	case rsmt2d.Col:
		return f.readCol(axisIdx)
	case rsmt2d.Row:
		return f.readRow(axisIdx)
	}
	return nil, fmt.Errorf("unknown axis")
}

func (f *EdsFile) readRow(idx int) ([]share.Share, error) {
	shrLn := int(f.hdr.shareSize)
	odsLn := int(f.hdr.squareSize)

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

func (f *EdsFile) readCol(idx int) ([]share.Share, error) {
	shrLn := int(f.hdr.shareSize)
	odsLn := int(f.hdr.squareSize)

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

func (f *EdsFile) Share(
	_ context.Context,
	axisType rsmt2d.Axis,
	axisIdx, shrIdx int,
) (share.Share, nmt.Proof, error) {
	shares, err := f.axis(axisType, axisIdx)
	if err != nil {
		return nil, nmt.Proof{}, err
	}

	tree := wrapper.NewErasuredNamespacedMerkleTree(uint64(f.Size()/2), uint(axisIdx))
	for _, shr := range shares {
		err := tree.Push(shr)
		if err != nil {
			return nil, nmt.Proof{}, err
		}
	}

	proof, err := tree.ProveRange(shrIdx, shrIdx+1)
	if err != nil {
		return nil, nmt.Proof{}, err
	}

	return shares[shrIdx], proof, nil
}

func (f *EdsFile) Data(_ context.Context, namespace share.Namespace, rowIdx int) (share.NamespacedRow, error) {
	shares, err := f.axis(rsmt2d.Row, rowIdx)
	if err != nil {
		return share.NamespacedRow{}, err
	}
	return ndDateFromShares(shares, namespace, rowIdx)
}

func (f *EdsFile) EDS(_ context.Context) (*rsmt2d.ExtendedDataSquare, error) {
	shrLn := int(f.hdr.shareSize)
	odsLn := int(f.hdr.squareSize)

	buf := make([]byte, odsLn*odsLn*shrLn)
	if _, err := f.fl.ReadAt(buf, HeaderSize); err != nil {
		return nil, err
	}

	shrs := make([][]byte, odsLn*odsLn)
	for i := 0; i < odsLn; i++ {
		for j := 0; j < odsLn; j++ {
			pos := i*odsLn + j
			shrs[pos] = buf[pos*shrLn : (pos+1)*shrLn]
		}
	}

	treeFn := wrapper.NewConstructor(uint64(f.hdr.squareSize / 2))
	return rsmt2d.ImportExtendedDataSquare(shrs, share.DefaultRSMT2DCodec(), treeFn)
}
