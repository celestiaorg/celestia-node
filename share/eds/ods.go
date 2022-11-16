package eds

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"

	cbor "github.com/ipfs/go-ipld-cbor"
	"github.com/ipld/go-car"
	"github.com/ipld/go-car/util"
)

var errNilReader = errors.New("odsReader: can't create ODSReadr over nil reader")

type odsReaderBuffered struct {
	reader                 *bufio.Reader
	current, odsSquareSize int
	buf                    *bytes.Buffer
	err                    error
}

// ODSReader reads data from io.ReadCloser and limits the reader to the CAR header and first quadrant (ODS)
func ODSReader(r io.ReadCloser) io.Reader {
	if r == nil {
		return &odsReaderBuffered{err: errNilReader}
	}

	odsR := &odsReaderBuffered{
		reader: bufio.NewReader(r),
		buf:    new(bytes.Buffer),
	}

	// read full header to determine amount of shares needed
	hb, err := util.LdRead(odsR.reader)
	if err != nil {
		odsR.err = fmt.Errorf("read header: %v", err)
		return odsR
	}

	var ch car.CarHeader
	if err := cbor.DecodeInto(hb, &ch); err != nil {
		odsR.err = fmt.Errorf("invalid header: %v", err)
		return odsR
	}

	odsWidth := len(ch.Roots) / 4
	odsR.odsSquareSize = odsWidth * odsWidth

	// save header for further reads
	odsR.err = util.LdWrite(odsR.buf, hb)

	return odsR
}

func (r *odsReaderBuffered) Read(p []byte) (n int, err error) {
	if r.err != nil {
		return 0, err
	}

	if r.buf.Len() > 0 {
		return r.buf.Read(p)
	}

	if r.current < r.odsSquareSize && r.buf.Len() < len(p) {
		data, err := util.LdRead(r.reader)
		if err != nil {
			return 0, err
		}

		err = util.LdWrite(r.buf, data)
		if err != nil {
			return 0, err
		}

		r.current++
	}
	// read buffer to slice
	return r.buf.Read(p)
}
