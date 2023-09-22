package eds

import (
	"fmt"
	"io"
	"os"

	"golang.org/x/exp/mmap"

	"github.com/celestiaorg/celestia-app/pkg/wrapper"
	"github.com/celestiaorg/nmt"
	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/share"
)

type FileConfig struct {
	Version     FileVersion
	Compression FileCompression
	Mode        FileMode

	// 	extensions  map[string]string
	// TODO: Add codec
}

// File
// * immutable
// * versionable
// TODO:
//   - Cache Rows and Cols
//   - Avoid storing constant shares, like padding
type File struct {
	path string
	hdr  *Header
	fl   fileBackend
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

	// TODO(WWondertan): Validate header
	return &File{
		path: path,
		hdr:  h,
		fl:   f,
	}, nil
}

func CreateFile(path string, eds *rsmt2d.ExtendedDataSquare, cfgs ...FileConfig) (*File, error) {
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

	return &File{
		path: path,
		fl:   f,
		hdr:  h,
	}, err
}

func (f *File) Close() error {
	return f.fl.Close()
}

func (f *File) Header() *Header {
	return f.hdr
}

func (f *File) Axis(idx int, axis rsmt2d.Axis) ([]share.Share, error) {
	shrLn := int(f.hdr.shareSize)
	sqrLn := int(f.hdr.squareSize)
	if f.Header().Config().Mode == ODSMode {
		sqrLn /= 2
	}

	shrs := make([]share.Share, sqrLn)
	switch axis {
	case rsmt2d.Col:
		for i := 0; i < sqrLn; i++ {
			pos := idx + i*sqrLn
			offset := pos*shrLn + HeaderSize

			shr := make(share.Share, shrLn)
			if _, err := f.fl.ReadAt(shr, int64(offset)); err != nil {
				return nil, err
			}
			shrs[i] = shr
		}
	case rsmt2d.Row:
		pos := idx * sqrLn
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

func (f *File) Share(idx int) (share.Share, error) {
	// TODO: Check the cache first
	shrLn := int64(f.hdr.shareSize)

	offset := int64(idx)*shrLn + HeaderSize
	shr := make(share.Share, shrLn)
	if _, err := f.fl.ReadAt(shr, offset); err != nil {
		return nil, err
	}
	return shr, nil
}

func (f *File) ShareWithProof(idx int, axis rsmt2d.Axis) (share.Share, nmt.Proof, error) {
	// TODO: Cache the axis as well as computed tree
	sqrLn := int(f.hdr.squareSize)
	axsIdx, shrIdx := idx/sqrLn, idx%sqrLn
	if axis == rsmt2d.Col {
		axsIdx, shrIdx = shrIdx, axsIdx
	}

	shrs, err := f.Axis(axsIdx, axis)
	if err != nil {
		return nil, nmt.Proof{}, err
	}

	tree := wrapper.NewErasuredNamespacedMerkleTree(uint64(sqrLn/2), uint(axsIdx))
	for _, shr := range shrs {
		err = tree.Push(shr)
		if err != nil {
			return nil, nmt.Proof{}, err
		}
	}

	proof, err := tree.ProveRange(shrIdx, shrIdx+1)
	if err != nil {
		return nil, nmt.Proof{}, err
	}

	return shrs[shrIdx], proof, nil
}

func (f *File) EDS() (*rsmt2d.ExtendedDataSquare, error) {
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
