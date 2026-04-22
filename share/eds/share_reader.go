package eds

import (
	"bytes"
	"errors"
	"fmt"
	"io"

	libshare "github.com/celestiaorg/go-square/v3/share"
)

// ShareReader implement io.Reader over general function that gets shares by
// their respective Row and Col coordinates.
// It enables share streaming over arbitrary storages.
type ShareReader struct {
	// getShare general share getting function for share retrieval
	getShare func(rowIdx, colIdx int) (libshare.Share, error)

	// buf buffers shares from partial reads with default size
	buf *bytes.Buffer
	// current is the amount of Shares stored in square that have been written by squareCopy. When
	// current reaches total, squareCopy will prevent further reads by returning io.EOF
	current, odsSize, total int
}

// NewShareReader constructs a new ShareGetter from underlying ODS size and general share getting function.
func NewShareReader(odsSize int, getShare func(rowIdx, colIdx int) (libshare.Share, error)) *ShareReader {
	return &ShareReader{
		getShare: getShare,
		buf:      bytes.NewBuffer(nil),
		odsSize:  odsSize,
		total:    odsSize * odsSize,
	}
}

func (r *ShareReader) Read(p []byte) (int, error) {
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
		rawShare := share.ToBytes()
		// copy share to provided buffer
		emptySpace := len(p) - written
		r.current++
		if len(rawShare) < emptySpace {
			n := copy(p[written:], rawShare)
			written += n
			continue
		}

		// if share didn't fit into buffer fully, store remaining bytes into inner buf
		n := copy(p[written:], rawShare[:emptySpace])
		written += n
		n, err = r.buf.Write(rawShare[emptySpace:])
		if err != nil {
			return 0, fmt.Errorf("write share to inner buffer: %w", err)
		}
		if n != len(rawShare)-emptySpace {
			return 0, fmt.Errorf("share was not written fully: %w", io.ErrShortWrite)
		}
		return written, nil
	}
	return written, nil
}
