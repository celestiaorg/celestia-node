package file

import (
	"encoding/binary"
	"fmt"
	"io"

	"github.com/celestiaorg/celestia-node/share"
)

type headerVersion uint8

const (
	headerVersionV0 headerVersion = 1
	headerVOSize    int           = 40
)

type headerV0 struct {
	fileVersion fileVersion
	fileType    fileType

	// Taken directly from EDS
	shareSize  uint16
	squareSize uint16

	datahash share.DataHash
}

type fileVersion uint8

const (
	fileV0 fileVersion = iota + 1
)

type fileType uint8

const (
	ODS fileType = iota
	q1q4
)

func readHeader(r io.Reader) (*headerV0, error) {
	// read first byte to determine the fileVersion
	var version headerVersion
	if err := binary.Read(r, binary.LittleEndian, &version); err != nil {
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
	n, err := w.Write([]byte{byte(headerVersionV0)})
	if err != nil {
		return fmt.Errorf("writeHeader: %w", err)
	}
	if n != 1 {
		return fmt.Errorf("writeHeader: wrote %d bytes, expected 1", n)
	}
	_, err = h.WriteTo(w)
	return err
}

func (h *headerV0) Size() int {
	// header size + 1 byte for header fileVersion
	return headerVOSize + 1
}

func (h *headerV0) WriteTo(w io.Writer) (int64, error) {
	buf := make([]byte, headerVOSize)
	buf[0] = byte(h.fileVersion)
	buf[1] = byte(h.fileType)
	binary.LittleEndian.PutUint16(buf[4:6], h.shareSize)
	binary.LittleEndian.PutUint16(buf[6:8], h.squareSize)
	copy(buf[8:40], h.datahash)
	n, err := w.Write(buf)
	return int64(n), err
}

func (h *headerV0) ReadFrom(r io.Reader) (int64, error) {
	bytesHeader := make([]byte, headerVOSize)
	n, err := io.ReadFull(r, bytesHeader)
	if n != headerVOSize {
		return 0, fmt.Errorf("headerV0 ReadFrom: read %d bytes, expected %d", len(bytesHeader), headerVOSize)
	}

	h.fileVersion = fileVersion(bytesHeader[0])
	h.fileType = fileType(bytesHeader[1])
	h.shareSize = binary.LittleEndian.Uint16(bytesHeader[4:6])
	h.squareSize = binary.LittleEndian.Uint16(bytesHeader[6:8])
	h.datahash = bytesHeader[8:40]
	return int64(headerVOSize), err
}
