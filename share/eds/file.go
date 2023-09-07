package eds

import (
	"io"
	"os"

	"golang.org/x/exp/mmap"

	"github.com/celestiaorg/celestia-app/pkg/wrapper"
	"github.com/celestiaorg/nmt"
	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/share"
)

// File
// * immutable
// * versionable
// TODO:
//   - Cache Rows and Cols
//   - Avoid storing constant shares, like padding
type File struct {
	path string
	h    Header
	f    fileBackend
}

type fileBackend interface {
	io.ReaderAt
	io.Closer
}

func OpenFile(path string) (*File, error) {
	f, err := mmap.Open(path)
	if err != nil {
		return nil, err
	}

	h, err := ReadHeaderAt(f, 0)
	if err != nil {
		return nil, err
	}

	return &File{
		path: path,
		h:    h,
		f:    f,
	}, nil
}

// TODO: Allow setting features
func CreateFile(path string, eds *rsmt2d.ExtendedDataSquare) (*File, error) {
	f, err := os.Create(path)
	if err != nil {
		return nil, err
	}

	h := Header{
		ShareSize:  uint16(len(eds.GetCell(0, 0))), // TODO: rsmt2d should expose this field
		SquareSize: uint32(eds.Width()),
	}

	if _, err = h.WriteTo(f); err != nil {
		return nil, err
	}

	for _, shr := range eds.Flattened() {
		// TOOD: Buffer and write as single?
		if _, err := f.Write(shr); err != nil {
			return nil, err
		}
	}

	return &File{
		path: path,
		f:    f,
		h:    h,
	}, err
}

func (f *File) Close() error {
	return f.f.Close()
}

func (f *File) Header() Header {
	return f.h
}

func (f *File) Axis(idx int, tp rsmt2d.Axis) ([]share.Share, error) {
	// TODO: Add Col support
	shrLn := int64(f.h.ShareSize)
	sqrLn := int64(f.h.SquareSize)
	rwwLn := shrLn * sqrLn

	offset := int64(idx)*rwwLn + HeaderSize
	rowdata := make([]byte, rwwLn)
	if _, err := f.f.ReadAt(rowdata, offset); err != nil {
		return nil, err
	}

	row := make([]share.Share, sqrLn)
	for i := range row {
		row[i] = rowdata[int64(i)*shrLn : (int64(i)+1)*shrLn]
	}
	return row, nil
}

func (f *File) Share(idx int) (share.Share, error) {
	// TODO: Check the cache first
	shrLn := int64(f.h.ShareSize)

	offset := int64(idx)*shrLn + HeaderSize
	shr := make(share.Share, shrLn)
	if _, err := f.f.ReadAt(shr, offset); err != nil {
		return nil, err
	}
	return shr, nil
}

func (f *File) ShareWithProof(idx int, axis rsmt2d.Axis) (share.Share, *nmt.Proof, error) {
	// TODO: Cache the axis as well as computed tree
	sqrLn := int(f.h.SquareSize)
	rowIdx := idx / sqrLn
	shrs, err := f.Axis(rowIdx, axis)
	if err != nil {
		return nil, nil, err
	}

	tree := wrapper.NewErasuredNamespacedMerkleTree(uint64(f.h.SquareSize/2), uint(rowIdx))
	for _, shr := range shrs {
		err = tree.Push(shr)
		if err != nil {
			return nil, nil, err
		}
	}

	shrIdx := idx % sqrLn
	proof, err := tree.ProveRange(shrIdx, shrIdx+1)
	if err != nil {
		return nil, nil, err
	}

	return shrs[shrIdx], &proof, nil
}

func (f *File) EDS() (*rsmt2d.ExtendedDataSquare, error) {
	shrLn := int(f.h.ShareSize)
	sqrLn := int(f.h.SquareSize)

	buf := make([]byte, sqrLn*sqrLn*shrLn)
	if _, err := f.f.ReadAt(buf, HeaderSize); err != nil {
		return nil, err
	}

	shrs := make([][]byte, sqrLn*sqrLn)
	for i := 0; i < sqrLn; i++ {
		for j := 0; j < sqrLn; j++ {
			coord := i*sqrLn + j
			shrs[coord] = buf[coord*shrLn : (coord+1)*shrLn]
		}
	}

	treeFn := func(_ rsmt2d.Axis, index uint) rsmt2d.Tree {
		tree := wrapper.NewErasuredNamespacedMerkleTree(uint64(f.h.SquareSize/2), index)
		return &tree
	}

	eds, err := rsmt2d.ImportExtendedDataSquare(shrs, share.DefaultRSMT2DCodec(), treeFn)
	if err != nil {
		return nil, err
	}

	return eds, nil
}
