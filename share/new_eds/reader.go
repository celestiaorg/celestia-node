package eds

import (
	"bytes"
	"errors"
	"fmt"
	"io"

	"github.com/celestiaorg/celestia-node/share"
)

func NewBufferedReader(w minCopy) *BufferedReader {
	return &BufferedReader{
		w:   w,
		buf: bytes.NewBuffer(nil),
	}
}

// BufferedReader will read Shares from inMemOds into the buffer.
// It exposes the buffer to be read by io.Reader interface implementation
type BufferedReader struct {
	w   minCopy
	buf *bytes.Buffer
}

func (r *BufferedReader) Read(p []byte) (int, error) {
	// if provided array is smaller than data in buf, read from buf
	if len(p) <= r.buf.Len() {
		return r.buf.Read(p)
	}

	// fill the buffer with data from writer
	remaining := len(p) - r.buf.Len()
	n, err := r.w.CopyAtLeast(r.buf, remaining)
	if err != nil && !errors.Is(err, io.EOF) {
		return n, err
	}
	// save remaining buffer for next read
	return r.buf.Read(p)
}

// ReadShares reads shares from the provided reader and constructs an Extended Data Square. Provided
// reader should contain shares in row-major order.
func ReadShares(r io.Reader, shareSize, odsSize int) ([]share.Share, error) {
	shares := make([]share.Share, odsSize*odsSize)
	var total int
	for i := range shares {
		share := make(share.Share, shareSize)
		n, err := io.ReadFull(r, share)
		if err != nil {
			return nil, fmt.Errorf("reading share: %w, bytes read: %v", err, total+n)
		}
		if n != shareSize {
			return nil, fmt.Errorf("share size mismatch: expected %v, got %v", shareSize, n)
		}
		shares[i] = share
		total += n
	}
	return shares, nil
}

// minCopy writes to provided writer at least min amount of bytes
type minCopy interface {
	CopyAtLeast(writer io.Writer, minAmount int) (int, error)
}
