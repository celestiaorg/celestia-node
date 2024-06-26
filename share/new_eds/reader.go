package eds

import (
	"bytes"
	"errors"
	"io"
)

func NewBufferedReader(w minWriterTo) *BufferedReader {
	return &BufferedReader{
		w:   w,
		buf: bytes.NewBuffer(nil),
	}
}

// BufferedReader will read Shares from inMemOds into the buffer.
// It exposes the buffer to be read by io.Reader interface implementation
type BufferedReader struct {
	w   minWriterTo
	buf *bytes.Buffer
}

func (r *BufferedReader) Read(p []byte) (int, error) {
	// if provided array is smaller than data in buf, read from buf
	if len(p) <= r.buf.Len() {
		return r.buf.Read(p)
	}

	// fill the buffer with data from writer
	min := len(p) - r.buf.Len()
	n, err := r.w.WriteTo(r.buf, min)
	if err != nil && !errors.Is(err, io.EOF) {
		return n, err
	}
	// save remaining buffer for next read
	return r.buf.Read(p)
}

// minWriterTo writes to provided writer at least min amount of bytes
type minWriterTo interface {
	WriteTo(writer io.Writer, minAmount int) (int, error)
}
