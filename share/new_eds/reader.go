package eds

import (
	"bytes"
	"errors"
	"fmt"
	"io"

	"github.com/celestiaorg/celestia-node/share"
)

func NewSharesReader(odsSize int, getShare func(rowIdx, colIdx int) ([]byte, error)) *BufferedReader {
	return &BufferedReader{
		getShare: getShare,
		buf:      bytes.NewBuffer(nil),
		odsSize:  odsSize,
		total:    odsSize * odsSize,
	}
}

// BufferedReader will read Shares from inMemOds into the buffer.
// It exposes the buffer to be read by io.Reader interface implementation
type BufferedReader struct {
	buf      *bytes.Buffer
	getShare func(rowIdx, colIdx int) ([]byte, error)
	// current is the amount of Shares stored in square that have been written by squareCopy. When
	// current reaches total, squareCopy will prevent further reads by returning io.EOF
	current, odsSize, total int
}

func (r *BufferedReader) Read(p []byte) (int, error) {
	if r.current >= r.total && r.buf.Len() == 0 {
		return 0, io.EOF
	}
	// if provided array is smaller than data in buf, read from buf
	if len(p) <= r.buf.Len() {
		return r.buf.Read(p)
	}
	n, err := io.ReadFull(r.buf, p)
	if err == nil {
		return n, nil
	}
	if !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
		return n, fmt.Errorf("unexpected error reading from buf: %w", err)
	}

	written := n
	for r.current < r.total {
		rowIdx, colIdx := r.current/r.odsSize, r.current%r.odsSize
		share, err := r.getShare(rowIdx, colIdx)
		if err != nil {
			return 0, fmt.Errorf("get share; %w", err)
		}

		// copy share to provided buffer
		emptySpace := len(p) - written
		r.current++
		if len(share) < emptySpace {
			n := copy(p[written:], share)
			written += n
			continue
		}

		// if share didn't fit into buffer fully, store remaining bytes into inner buf
		n := copy(p[written:], share[:emptySpace])
		written += n
		n, err = r.buf.Write(share[emptySpace:])
		if err != nil {
			return 0, fmt.Errorf("write share to inner buffer: %w", err)
		}
		if n != len(share)-emptySpace {
			return 0, fmt.Errorf("share was not written fully: %w", io.ErrShortWrite)
		}
		return written, nil
	}
	return written, nil
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
