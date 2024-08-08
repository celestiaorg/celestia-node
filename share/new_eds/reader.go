package eds

import (
	"bytes"
	"errors"
	"fmt"
	"io"
)

// BufferedReader will read Shares from getShare function into the buffer.
// It exposes the buffer to be read by io.Reader interface implementation
type BufferedReader struct {
	buf      *bytes.Buffer
	getShare func(rowIdx, colIdx int) ([]byte, error)
	// current is the amount of Shares stored in square that have been written by squareCopy. When
	// current reaches total, squareCopy will prevent further reads by returning io.EOF
	current, odsSize, total int
}

func NewSharesReader(odsSize int, getShare func(rowIdx, colIdx int) ([]byte, error)) *BufferedReader {
	return &BufferedReader{
		getShare: getShare,
		buf:      bytes.NewBuffer(nil),
		odsSize:  odsSize,
		total:    odsSize * odsSize,
	}
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
			return 0, fmt.Errorf("get share: %w", err)
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
