package file

import (
	"bytes"
	"encoding/binary"
	"io"

	"github.com/celestiaorg/celestia-node/share"
)

const HeaderSize = 64

type Header struct {
	version  fileVersion
	fileType fileType

	// Taken directly from EDS
	shareSize  uint16
	squareSize uint16

	// TODO(@walldiss) store all heights in the header?
	//heightы   []uint64
	datahash share.DataHash
}

type fileVersion uint8

const (
	FileV0 fileVersion = iota
)

type fileType uint8

const (
	ods fileType = iota
	q1q4
)

func (h *Header) Version() fileVersion {
	return h.version
}

func (h *Header) ShareSize() int {
	return int(h.shareSize)
}

func (h *Header) SquareSize() int {
	return int(h.squareSize)
}

func (h *Header) DataHash() share.DataHash {
	return h.datahash
}

func (h *Header) WriteTo(w io.Writer) (int64, error) {
	buf := make([]byte, HeaderSize)
	buf[0] = byte(h.version)
	buf[1] = byte(h.fileType)
	binary.LittleEndian.PutUint16(buf[2:4], h.shareSize)
	binary.LittleEndian.PutUint16(buf[4:6], h.squareSize)
	copy(buf[32:64], h.datahash)
	_, err := io.Copy(w, bytes.NewBuffer(buf))
	return HeaderSize, err
}

func ReadHeader(r io.Reader) (*Header, error) {
	buf := make([]byte, HeaderSize)
	_, err := io.ReadFull(r, buf)
	if err != nil {
		return nil, err
	}

	h := &Header{
		version:    fileVersion(buf[0]),
		fileType:   fileType(buf[1]),
		shareSize:  binary.LittleEndian.Uint16(buf[2:4]),
		squareSize: binary.LittleEndian.Uint16(buf[4:6]),
		datahash:   make([]byte, 32),
	}

	copy(h.datahash, buf[32:64])
	return h, err
}
