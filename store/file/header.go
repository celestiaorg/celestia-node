package file

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"

	"github.com/celestiaorg/celestia-node/share"
)

const headerSize = 64

type header struct {
	version  fileVersion
	fileType fileType

	// Taken directly from EDS
	shareSize  uint16
	squareSize uint16

	datahash share.DataHash
}

type fileVersion uint8

const (
	fileV0 fileVersion = iota
)

type fileType uint8

const (
	ods fileType = iota
	q1q4
)

func (h *header) WriteTo(w io.Writer) (int64, error) {
	b := bytes.NewBuffer(make([]byte, 0, headerSize))
	_ = b.WriteByte(byte(h.version))
	_ = b.WriteByte(byte(h.fileType))
	_ = binary.Write(b, binary.LittleEndian, h.shareSize)
	_ = binary.Write(b, binary.LittleEndian, h.squareSize)
	_, _ = b.Write(h.datahash)
	// write padding
	_, _ = b.Write(make([]byte, headerSize-b.Len()-1))
	return writeLenEncoded(w, b.Bytes())
}

func readHeader(r io.Reader) (*header, error) {
	bytesHeader, err := readLenEncoded(r)
	if err != nil {
		return nil, err
	}
	if len(bytesHeader) != headerSize-1 {
		return nil, fmt.Errorf("readHeader: read %d bytes, expected %d", len(bytesHeader), headerSize)
	}
	h := &header{
		version:    fileVersion(bytesHeader[0]),
		fileType:   fileType(bytesHeader[1]),
		shareSize:  binary.LittleEndian.Uint16(bytesHeader[2:4]),
		squareSize: binary.LittleEndian.Uint16(bytesHeader[4:6]),
		datahash:   make([]byte, 32),
	}

	copy(h.datahash, bytesHeader[6:6+32])
	return h, err
}

func writeLenEncoded(w io.Writer, data []byte) (int64, error) {
	_, err := w.Write([]byte{byte(len(data))})
	if err != nil {
		return 0, err
	}
	return io.Copy(w, bytes.NewBuffer(data))
}

func readLenEncoded(r io.Reader) ([]byte, error) {
	lenBuf := make([]byte, 1)
	_, err := io.ReadFull(r, lenBuf)
	if err != nil {
		return nil, err
	}

	data := make([]byte, lenBuf[0])
	n, err := io.ReadFull(r, data)
	if err != nil {
		return nil, err
	}
	if n != len(data) {
		return nil, fmt.Errorf("readLenEncoded: read %d bytes, expected %d", n, len(data))
	}
	return data, nil
}
