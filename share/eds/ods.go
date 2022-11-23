package eds

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"

	cbor "github.com/ipfs/go-ipld-cbor"
	"github.com/ipld/go-car"
	"github.com/ipld/go-car/util"
)

var errNilReader = errors.New("ods-reader: can't create ODSReader over nil reader")

type odsReaderBuffered struct {
	reader                 *bufio.Reader
	current, odsSquareSize int
	buf                    *bytes.Buffer
}

// ODSReader reads data from io.ReadCloser and limits the reader to the CAR header and first quadrant (ODS)
func ODSReader(r io.ReadCloser) (io.Reader, error) {
	if r == nil {
		return nil, errNilReader
	}

	odsR := &odsReaderBuffered{
		reader: bufio.NewReaderSize(r, 4),
		buf:    new(bytes.Buffer),
	}

	// first LdRead reads the full CAR header to determine amount of shares in the ODS
	data, err := util.LdRead(odsR.reader)
	if err != nil {
		return nil, fmt.Errorf("reading header: %v", err)
	}

	var header car.CarHeader
	err = cbor.DecodeInto(data, &header)
	if err != nil {
		return nil, fmt.Errorf("invalid header: %v", err)
	}

	// car header contains both row roots and col roots which is why
	// we divide by 4 to get the ODSWidth
	odsWidth := len(header.Roots) / 4
	odsR.odsSquareSize = odsWidth * odsWidth

	// NewCarReader will expect read header first, so save header for further reads
	return odsR, util.LdWrite(odsR.buf, data)
}

func (r *odsReaderBuffered) Read(p []byte) (n int, err error) {
	if r.buf.Len() > len(p) {
		return r.buf.Read(p)
	}

	if r.current < r.odsSquareSize && r.buf.Len() < len(p) {
		if err := r.readLeaf(); err != nil {
			return 0, err
		}

		r.current++
	}

	// read buffer to slice
	return r.buf.Read(p)
}

func (r *odsReaderBuffered) readLeaf() error {
	if _, err := r.reader.Peek(1); err != nil { // no more blocks, likely clean io.EOF
		return err
	}

	l, err := binary.ReadUvarint(r.reader)
	if err != nil {
		if err == io.EOF {
			return io.ErrUnexpectedEOF // don't silently pretend this is a clean EOF
		}
		return err
	}

	if l > uint64(util.MaxAllowedSectionSize) { // Don't OOM
		return errors.New("malformed car; header is bigger than util.MaxAllowedSectionSize")
	}

	buf := make([]byte, 8)
	n := binary.PutUvarint(buf, l)
	r.buf.Write(buf[:n])

	_, err = r.buf.ReadFrom(io.LimitReader(r.reader, int64(l)))

	return err
}
