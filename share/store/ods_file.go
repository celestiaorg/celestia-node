package store

import (
	"context"
	"fmt"
	"io"
	"os"
	"sync"

	"golang.org/x/sync/errgroup"

	"github.com/celestiaorg/celestia-app/pkg/wrapper"
	"github.com/celestiaorg/nmt"
	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/share"
)

var _ File = (*OdsFile)(nil)

type OdsFile struct {
	path string
	hdr  *Header
	fl   *os.File

	memPool memPool
}

type fileBackend interface {
	io.ReaderAt
	io.Closer
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

	// TODO(WWondertan): Validate header
	return &OdsFile{
		path: path,
		hdr:  h,
		fl:   f,
	}, nil
}

type memPool struct {
	codec       rsmt2d.Codec
	shares, ods *sync.Pool
}

func newMemPool(codec rsmt2d.Codec, size int) memPool {
	shares := &sync.Pool{
		New: func() interface{} {
			shrs := make([][]share.Share, size)
			for i := 0; i < size; i++ {
				if shrs[i] == nil {
					shrs[i] = make([]share.Share, size)
				}
			}
			return shrs
		},
	}

	ods := &sync.Pool{
		New: func() interface{} {
			buf := make([]byte, size*share.Size)
			return buf
		},
	}
	return memPool{
		shares: shares,
		ods:    ods,
		codec:  codec,
	}
}

func CreateOdsFile(path string, eds *rsmt2d.ExtendedDataSquare, memPool memPool) (*OdsFile, error) {
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

	for i := uint(0); i < eds.Width()/2; i++ {
		for j := uint(0); j < eds.Width()/2; j++ {
			// TODO: Implemented buffered write through io.CopyBuffer
			shr := eds.GetCell(i, j)
			if _, err := f.Write(shr); err != nil {
				return nil, err
			}
		}
	}

	return &OdsFile{
		path:    path,
		fl:      f,
		hdr:     h,
		memPool: memPool,
	}, f.Sync()
}

func (f *OdsFile) Size() int {
	return f.hdr.SquareSize()
}

func (f *OdsFile) Close() error {
	return f.fl.Close()
}

func (f *OdsFile) Header() *Header {
	return f.hdr
}

func (f *OdsFile) AxisHalf(ctx context.Context, axisType rsmt2d.Axis, axisIdx int) ([]share.Share, error) {
	// read axis from file if axis is in the first quadrant
	if axisIdx < f.Size()/2 {
		return f.odsAxisHalf(axisType, axisIdx)
	}

	ods := f.readOds(oppositeAxis(axisType))
	defer f.memPool.shares.Put(ods.shares)

	return computeAxisHalf(ctx, ods, f.memPool.codec, axisType, axisIdx)
}

func (f *OdsFile) odsAxisHalf(axisType rsmt2d.Axis, axisIdx int) ([]share.Share, error) {
	switch axisType {
	case rsmt2d.Col:
		return f.readCol(axisIdx)
	case rsmt2d.Row:
		return f.readRow(axisIdx)
	}
	return nil, fmt.Errorf("unknown axis")
}

type odsInMemFile struct {
	File
	axisType rsmt2d.Axis
	shares   [][]share.Share
}

func (f *odsInMemFile) Size() int {
	return len(f.shares) * 2
}

func (f *odsInMemFile) AxisHalf(_ context.Context, axisType rsmt2d.Axis, axisIdx int) ([]share.Share, error) {
	if axisType != f.axisType {
		return nil, fmt.Errorf("order of shares is not preserved")
	}
	return f.shares[axisIdx], nil
}

func (f *OdsFile) readOds(axisType rsmt2d.Axis) *odsInMemFile {
	shrLn := int(f.hdr.shareSize)
	odsLn := int(f.hdr.squareSize) / 2

	buf := f.memPool.ods.Get().([]byte)
	defer f.memPool.ods.Put(buf)

	shrs := f.memPool.shares.Get().([][]share.Share)
	for i := 0; i < odsLn; i++ {
		pos := HeaderSize + odsLn*shrLn*i
		if _, err := f.fl.ReadAt(buf, int64(pos)); err != nil {
			return nil
		}

		for j := 0; j < odsLn; j++ {
			if axisType == rsmt2d.Row {
				shrs[i][j] = buf[j*shrLn : (j+1)*shrLn]
			} else {
				shrs[j][i] = buf[j*shrLn : (j+1)*shrLn]
			}
		}
	}

	return &odsInMemFile{
		axisType: axisType,
		shares:   shrs,
	}
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

func (f *OdsFile) readCol(idx int) ([]share.Share, error) {
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

func computeAxisHalf(
	ctx context.Context,
	f File,
	codec rsmt2d.Codec,
	axisType rsmt2d.Axis,
	axisIdx int,
) ([]share.Share, error) {
	shares := make([]share.Share, f.Size()/2)

	// extend opposite half of the square while collecting shares for the first half of required axis
	g, ctx := errgroup.WithContext(ctx)
	opposite := oppositeAxis(axisType)
	for i := 0; i < f.Size()/2; i++ {
		i := i
		g.Go(func() error {
			original, err := f.AxisHalf(ctx, opposite, i)
			if err != nil {
				return err
			}

			parity, err := codec.Encode(original)
			if err != nil {
				return err
			}
			shares[i] = parity[axisIdx-f.Size()/2]
			return nil
		})
	}

	err := g.Wait()
	return shares, err
}

func (f *OdsFile) axis(ctx context.Context, axisType rsmt2d.Axis, axisIdx int) ([]share.Share, error) {
	original, err := f.AxisHalf(ctx, axisType, axisIdx)
	if err != nil {
		return nil, err
	}

	return extendShares(original)
}

func extendShares(original []share.Share) ([]share.Share, error) {
	parity, err := rsmt2d.NewLeoRSCodec().Encode(original)
	if err != nil {
		return nil, err
	}

	shares := make([]share.Share, 0, len(original)+len(parity))
	shares = append(shares, original...)
	shares = append(shares, parity...)

	return shares, nil
}

func (f *OdsFile) Share(
	ctx context.Context,
	axisType rsmt2d.Axis,
	axisIdx, shrIdx int,
) (share.Share, nmt.Proof, error) {
	shares, err := f.axis(ctx, axisType, axisIdx)
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

func (f *OdsFile) Data(ctx context.Context, namespace share.Namespace, rowIdx int) (share.NamespacedRow, error) {
	shares, err := f.axis(ctx, rsmt2d.Row, rowIdx)
	if err != nil {
		return share.NamespacedRow{}, err
	}
	return ndDateFromShares(shares, namespace, rowIdx)
}

func (f *OdsFile) EDS(_ context.Context) (*rsmt2d.ExtendedDataSquare, error) {
	shrLn := int(f.hdr.shareSize)
	odsLn := int(f.hdr.squareSize) / 2

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
	return rsmt2d.ComputeExtendedDataSquare(shrs, share.DefaultRSMT2DCodec(), treeFn)
}

func oppositeAxis(axis rsmt2d.Axis) rsmt2d.Axis {
	if axis == rsmt2d.Col {
		return rsmt2d.Row
	}
	return rsmt2d.Col
}
