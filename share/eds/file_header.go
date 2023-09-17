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
	Version     uint8
	Compression uint8
	Extensions  map[string]string
	// Taken directly from EDS
	ShareSize  uint16
	SquareSize uint32
}

func (h *Header) WriteTo(w io.Writer) (int64, error) {
	buf := make([]byte, HeaderSize)
	buf[0] = h.Version
	buf[1] = h.Compression
	binary.LittleEndian.PutUint16(buf[2:4], h.ShareSize)
	binary.LittleEndian.PutUint32(buf[4:12], h.SquareSize)
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

	h.Version = buf[0]
	h.Compression = buf[1]
	h.ShareSize = binary.LittleEndian.Uint16(buf[2:4])
	h.SquareSize = binary.LittleEndian.Uint32(buf[4:12])

	// TODO: Extensions
	return int64(n), err
}

func ReadHeaderAt(r io.ReaderAt, offset int64) (Header, error) {
	h := Header{}
	buf := make([]byte, HeaderSize)
	_, err := r.ReadAt(buf, offset)
	if err != nil {
		return h, err
	}

	h.Version = buf[0]
	h.Compression = buf[1]
	h.ShareSize = binary.LittleEndian.Uint16(buf[2:4])
	h.SquareSize = binary.LittleEndian.Uint32(buf[4:12])
	return h, nil
}
