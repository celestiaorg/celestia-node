package eds

import (
	"context"
	"fmt"
	"io"
	"math/rand"
	"os"

	"golang.org/x/exp/mmap"

	"github.com/celestiaorg/celestia-app/pkg/wrapper"
	"github.com/celestiaorg/nmt"
	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/ipld"
)

// TODO: remove
//type File interface {
//	io.Closer
//	Size() int
//	ShareWithProof(xisIdx, shrIdx int) (share.Share, nmt.Proof, rsmt2d.Axis, error)
//	Axis(axisType rsmt2d.Axis, axisIdx int) ([]share.Share, error)
//	AxisHalf(axisType rsmt2d.Axis, axisIdx int) ([]share.Share, error)
//	Data(namespace share.Namespace, axisIdx int) ([]share.Share, nmt.Proof, error)
//	EDS() (*rsmt2d.ExtendedDataSquare, error)
//}

type FileConfig struct {
	Version     FileVersion
	Compression FileCompression
	Mode        FileMode

	// 	extensions  map[string]string
	// TODO: Add codec
}

// LazyFile
// * immutable
// * versionable
// TODO:
//   - Cache Rows and Cols
//   - Avoid storing constant shares, like padding
type LazyFile struct {
	path string
	hdr  *Header
	fl   fileBackend
}

type fileBackend interface {
	io.ReaderAt
	io.Closer
}

func OpenFile(path string) (*LazyFile, error) {
	f, err := mmap.Open(path)
	if err != nil {
		return nil, err
	}

	h, err := ReadHeaderAt(f, 0)
	if err != nil {
		return nil, err
	}

	// TODO(WWondertan): Validate header
	return &LazyFile{
		path: path,
		hdr:  h,
		fl:   f,
	}, nil
}

func CreateFile(path string, eds *rsmt2d.ExtendedDataSquare, cfgs ...FileConfig) (*LazyFile, error) {
	f, err := os.Create(path)
	if err != nil {
		return nil, err
	}

	cfg := FileConfig{}
	if cfgs != nil {
		cfg = cfgs[0]
	}

	h := &Header{
		shareSize:  uint16(len(eds.GetCell(0, 0))), // TODO: rsmt2d should expose this field
		squareSize: uint32(eds.Width()),
		cfg:        cfg,
	}

	if _, err = h.WriteTo(f); err != nil {
		return nil, err
	}

	width := eds.Width()
	if cfg.Mode == ODSMode {
		width /= 2
	}
	for i := uint(0); i < width; i++ {
		for j := uint(0); j < width; j++ {
			// TODO: Buffer and write as single?
			shr := eds.GetCell(i, j)
			if _, err := f.Write(shr); err != nil {
				return nil, err
			}
		}
	}

	return &LazyFile{
		path: path,
		fl:   f,
		hdr:  h,
	}, f.Sync()
}

func (f *LazyFile) Size() int {
	return f.hdr.SquareSize()
}

func (f *LazyFile) Close() error {
	return f.fl.Close()
}

func (f *LazyFile) Header() *Header {
	return f.hdr
}

func (f *LazyFile) Axis(axisType rsmt2d.Axis, axisIdx int) ([]share.Share, error) {
	shrLn := int(f.hdr.shareSize)
	sqrLn := int(f.hdr.squareSize)
	if f.Header().Config().Mode == ODSMode {
		sqrLn /= 2
	}

	shrs := make([]share.Share, sqrLn)
	switch axisType {
	case rsmt2d.Col:
		for i := 0; i < sqrLn; i++ {
			pos := axisIdx + i*sqrLn
			offset := pos*shrLn + HeaderSize

			shr := make(share.Share, shrLn)
			if _, err := f.fl.ReadAt(shr, int64(offset)); err != nil {
				return nil, err
			}
			shrs[i] = shr
		}
	case rsmt2d.Row:
		pos := axisIdx * sqrLn
		offset := pos*shrLn + HeaderSize

		axsData := make([]byte, sqrLn*shrLn)
		if _, err := f.fl.ReadAt(axsData, int64(offset)); err != nil {
			return nil, err
		}

		for i := range shrs {
			shrs[i] = axsData[i*shrLn : (i+1)*shrLn]
		}
	default:
		return nil, fmt.Errorf("unknown axis")
	}

	if f.Header().Config().Mode == ODSMode {
		parity, err := share.DefaultRSMT2DCodec().Decode(shrs)
		if err != nil {
			return nil, err
		}

		return append(shrs, parity...), nil
	}
	return shrs, nil
}

func (f *LazyFile) AxisHalf(axisType rsmt2d.Axis, axisIdx int) ([]share.Share, error) {
	// TODO(@Wondertan): this has to read directly from the file, avoiding recompute
	fullAxis, err := f.Axis(axisType, axisIdx)
	if err != nil {
		return nil, err
	}

	return fullAxis[:len(fullAxis)/2], nil
}

func (f *LazyFile) ShareWithProof(axisIdx, shrIdx int) (share.Share, nmt.Proof, rsmt2d.Axis, error) {
	// TODO: Cache the axis as well as computed tree
	axisType := rsmt2d.Row
	if rand.Int()/2 == 0 {
		axisType = rsmt2d.Col
	}
	sqrLn := int(f.hdr.squareSize)
	shrs, err := f.Axis(axisType, axisIdx)
	if err != nil {
		return nil, nmt.Proof{}, axisType, err
	}

	tree := wrapper.NewErasuredNamespacedMerkleTree(uint64(sqrLn/2), uint(axisIdx))
	for _, shr := range shrs {
		err = tree.Push(shr)
		if err != nil {
			return nil, nmt.Proof{}, axisType, err
		}
	}

	proof, err := tree.ProveRange(shrIdx, shrIdx+1)
	if err != nil {
		return nil, nmt.Proof{}, axisType, err
	}

	return shrs[shrIdx], proof, axisType, nil
}

func (f *LazyFile) Data(namespace share.Namespace, axisIdx int) ([]share.Share, nmt.Proof, error) {
	shrs, err := f.Axis(rsmt2d.Row, axisIdx)
	if err != nil {
		return nil, nmt.Proof{}, err
	}

	return NDFromShares(shrs, namespace, axisIdx)
}

func (f *LazyFile) EDS() (*rsmt2d.ExtendedDataSquare, error) {
	shrLn := int(f.hdr.shareSize)
	sqrLn := int(f.hdr.squareSize)
	if f.Header().Config().Mode == ODSMode {
		sqrLn /= 2
	}

	buf := make([]byte, sqrLn*sqrLn*shrLn)
	if _, err := f.fl.ReadAt(buf, HeaderSize); err != nil {
		return nil, err
	}

	shrs := make([][]byte, sqrLn*sqrLn)
	for i := 0; i < sqrLn; i++ {
		for j := 0; j < sqrLn; j++ {
			pos := i*sqrLn + j
			shrs[pos] = buf[pos*shrLn : (pos+1)*shrLn]
		}
	}

	codec := share.DefaultRSMT2DCodec()
	treeFn := wrapper.NewConstructor(uint64(f.hdr.squareSize / 2))

	switch f.Header().Config().Mode {
	case EDSMode:
		return rsmt2d.ImportExtendedDataSquare(shrs, codec, treeFn)
	case ODSMode:
		return rsmt2d.ComputeExtendedDataSquare(shrs, codec, treeFn)
	default:
		return nil, fmt.Errorf("invalid mode type") // TODO(@Wondertan): Do fields validation right after read
	}
}

func NDFromShares(shrs []share.Share, namespace share.Namespace, axisIdx int) ([]share.Share, nmt.Proof, error) {
	bserv := ipld.NewMemBlockservice()
	batchAdder := ipld.NewNmtNodeAdder(context.TODO(), bserv, ipld.MaxSizeBatchOption(len(shrs)))
	tree := wrapper.NewErasuredNamespacedMerkleTree(uint64(len(shrs)/2), uint(axisIdx),
		nmt.NodeVisitor(batchAdder.Visit))
	for _, shr := range shrs {
		err := tree.Push(shr)
		if err != nil {
			return nil, nmt.Proof{}, err
		}
	}

	root, err := tree.Root()
	if err != nil {
		return nil, nmt.Proof{}, err
	}

	err = batchAdder.Commit()
	if err != nil {
		return nil, nmt.Proof{}, err
	}

	cid := ipld.MustCidFromNamespacedSha256(root)
	row, proof, err := ipld.GetSharesByNamespace(context.TODO(), bserv, cid, namespace, len(shrs))
	if err != nil {
		return nil, nmt.Proof{}, err
	}
	return row, *proof, nil
}
