package file

import (
	"encoding/binary"
	"fmt"
	"io"

	"github.com/celestiaorg/celestia-node/share"
)

// headerVOSize is the size of the headerV0 in bytes. It has more space than the headerV0 struct
// to allow for future extensions of the header without breaking compatibility.
const headerVOSize int = 64

type headerVersion uint8

const headerVersionV0 headerVersion = iota + 1

type headerV0 struct {
	fileVersion fileVersion

	// Taken directly from EDS
	shareSize  uint16
	squareSize uint16

	datahash share.DataHash
}

type fileVersion uint8

const (
	fileV0 fileVersion = iota + 1
)

func readHeader(r io.Reader) (*headerV0, error) {
	// read first byte to determine the fileVersion
	var version headerVersion
	err := binary.Read(r, binary.LittleEndian, &version)
	if err != nil {
		return nil, fmt.Errorf("readHeader: %w", err)
	}

	switch version {
	case headerVersionV0:
		h := &headerV0{}
		_, err := h.ReadFrom(r)
		return h, err
	default:
		return nil, fmt.Errorf("unsupported header fileVersion: %d", version)
	}
}

func writeHeader(w io.Writer, h *headerV0) error {
	err := binary.Write(w, binary.LittleEndian, headerVersionV0)
	if err != nil {
		return fmt.Errorf("writeHeader: %w", err)
	}
	_, err = h.WriteTo(w)
	return err
}

func (h *headerV0) SquareSize() int {
	return int(h.squareSize)
}

func (h *headerV0) ShareSize() int {
	return int(h.shareSize)
}

func (h *headerV0) Size() int {
	// header size + 1 byte for header fileVersion
	return headerVOSize + 1
}

func (h *headerV0) RootsSize() int {
	// axis roots are stored in two parts: row roots and column roots, each part has size equal to
	// the square size. Thus, the total amount of roots is equal to the square size * 2.
	return share.AxisRootSize * h.SquareSize() * 2
}

func (h *headerV0) OffsetWithRoots() int {
	return h.RootsSize() + h.Size()
}

func (h *headerV0) WriteTo(w io.Writer) (int64, error) {
	buf := make([]byte, headerVOSize)
	buf[0] = byte(h.fileVersion)
	binary.LittleEndian.PutUint16(buf[28:30], h.shareSize)
	binary.LittleEndian.PutUint16(buf[30:32], h.squareSize)
	copy(buf[32:64], h.datahash)
	n, err := w.Write(buf)
	return int64(n), err
}

func (h *headerV0) ReadFrom(r io.Reader) (int64, error) {
	bytesHeader := make([]byte, headerVOSize)
	n, err := io.ReadFull(r, bytesHeader)
	if n != headerVOSize {
		return 0, fmt.Errorf("headerV0 ReadFrom: read %d bytes, expected %d", len(bytesHeader), headerVOSize)
	}
	if len(bytesHeader) == 0 {
		return 0, fmt.Errorf("headerV0 ReadFrom: empty header")
	}

	h.fileVersion = fileVersion(bytesHeader[0])
	h.shareSize = binary.LittleEndian.Uint16(bytesHeader[28:30])
	h.squareSize = binary.LittleEndian.Uint16(bytesHeader[30:32])
	h.datahash = bytesHeader[32:64]
	return int64(headerVOSize), err
}
