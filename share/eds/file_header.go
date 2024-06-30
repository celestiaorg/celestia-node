package eds

import (
	"encoding/binary"
	"io"
)

const HeaderSize = 32

type Header struct {
	// User set features
	cfg FileConfig

	// Taken directly from EDS
	shareSize  uint16
	squareSize uint32
}

func (h *Header) Config() FileConfig {
	return h.cfg
}

func (h *Header) ShareSize() int {
	return int(h.shareSize)
}

func (h *Header) SquareSize() int {
	return int(h.squareSize)
}

func (h *Header) WriteTo(w io.Writer) (int64, error) {
	buf := make([]byte, HeaderSize)
	buf[0] = byte(h.Config().Version)
	buf[1] = byte(h.Config().Compression)
	buf[2] = byte(h.Config().Mode)
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

	h.cfg.Version = FileVersion(buf[0])
	h.cfg.Compression = FileCompression(buf[1])
	h.cfg.Mode = FileMode(buf[2])
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

	h.cfg.Version = FileVersion(buf[0])
	h.cfg.Compression = FileCompression(buf[1])
	h.cfg.Mode = FileMode(buf[2])
	h.shareSize = binary.LittleEndian.Uint16(buf[2:4])
	h.squareSize = binary.LittleEndian.Uint32(buf[4:12])
	return h, nil
}
