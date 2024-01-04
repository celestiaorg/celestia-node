package store

import (
	"encoding/binary"
	"io"
)

const HeaderSize = 32

type Header struct {
	version FileVersion

	// Taken directly from EDS
	shareSize  uint16
	squareSize uint16
}

type FileVersion uint8

const (
	FileV0 FileVersion = iota
)

func (h *Header) Version() FileVersion {
	return h.version
}

func (h *Header) ShareSize() int {
	return int(h.shareSize)
}

func (h *Header) SquareSize() int {
	return int(h.squareSize)
}

func (h *Header) WriteTo(w io.Writer) (int64, error) {
	buf := make([]byte, HeaderSize)
	buf[0] = byte(h.version)
	binary.LittleEndian.PutUint16(buf[1:3], h.shareSize)
	binary.LittleEndian.PutUint16(buf[3:5], h.squareSize)
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

	h.version = FileVersion(buf[0])
	h.shareSize = binary.LittleEndian.Uint16(buf[1:3])
	h.squareSize = binary.LittleEndian.Uint16(buf[3:5])

	// TODO: Extensions
	return int64(n), err
}

func ReadHeader(r io.ReaderAt) (*Header, error) {
	h := &Header{}
	buf := make([]byte, HeaderSize)
	_, err := r.ReadAt(buf, 0)
	if err != nil {
		return h, err
	}

	h.version = FileVersion(buf[0])
	h.shareSize = binary.LittleEndian.Uint16(buf[1:3])
	h.squareSize = binary.LittleEndian.Uint16(buf[3:5])
	return h, nil
}
