package eds

import (
	"encoding/binary"
	"io"
)

const HeaderSize = 32

type Header struct {
	// User set features
	// TODO: Add codec
	// TDOD: Add ODS support
	version     uint8
	compression uint8
	// 	extensions  map[string]string

	// Taken directly from EDS
	shareSize  uint16
	squareSize uint32
}

func (h *Header) ShareSize() int {
	return int(h.shareSize)
}

func (h *Header) SquareSize() int {
	return int(h.squareSize)
}

// TODO(@Wondertan) Should return special types
func (h *Header) Version() uint8 {
	return h.version
}
func (h *Header) Compression() uint8 {
	return h.compression
}

func (h *Header) WriteTo(w io.Writer) (int64, error) {
	buf := make([]byte, HeaderSize)
	buf[0] = h.version
	buf[1] = h.compression
	binary.LittleEndian.PutUint16(buf[2:4], h.shareSize)
	binary.LittleEndian.PutUint32(buf[4:12], h.squareSize)
	// TODO: Extensions
	n, err := w.Write(buf)
	return int64(n), err
}

func (h *Header) ReadFrom(r io.Reader) (int64, error) {
	buf := make([]byte, HeaderSize)
	n, err := io.ReadFull(r, buf)
	if err != nil {
		return int64(n), err
	}

	h.version = buf[0]
	h.compression = buf[1]
	h.shareSize = binary.LittleEndian.Uint16(buf[2:4])
	h.squareSize = binary.LittleEndian.Uint32(buf[4:12])

	// TODO: Extensions
	return int64(n), err
}

func ReadHeaderAt(r io.ReaderAt, offset int64) (*Header, error) {
	h := &Header{}
	buf := make([]byte, HeaderSize)
	_, err := r.ReadAt(buf, offset)
	if err != nil {
		return h, err
	}

	h.version = buf[0]
	h.compression = buf[1]
	h.shareSize = binary.LittleEndian.Uint16(buf[2:4])
	h.squareSize = binary.LittleEndian.Uint32(buf[4:12])
	return h, nil
}
