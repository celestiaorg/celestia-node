package shwap

import (
	"bytes"
	"context"
	"fmt"
	"io"

	libshare "github.com/celestiaorg/go-square/v4/share"
)

// rowStreamMemoryMultiplier covers shares, NMT construction and encoding buffers.
const rowStreamMemoryMultiplier = 8

// rowStreamReserve grows with row width, not the number of requested rows.
func rowStreamReserve(edsSize int) int {
	return rowStreamMemoryMultiplier*edsSize*libshare.ShareSize + edsStreamBufferSize
}

// rowStreamReader builds the next row only after the previous row has been consumed.
type rowStreamReader struct {
	ctx       context.Context
	next      func(int) (RowNamespaceData, error)
	rows, pos int
	buf       bytes.Buffer
	err       error
}

func newRowStreamReader(ctx context.Context, rows int, next func(int) (RowNamespaceData, error)) (io.Reader, error) {
	r := &rowStreamReader{ctx: ctx, rows: rows, next: next}
	// Read the first row before the server sends OK.
	if rows > 0 {
		if err := r.load(); err != nil {
			return nil, err
		}
	}
	return r, nil
}

func (r *rowStreamReader) load() error {
	if err := r.ctx.Err(); err != nil {
		return err
	}
	row, err := r.next(r.pos)
	if err != nil {
		return fmt.Errorf("reading row %d: %w", r.pos, err)
	}
	r.buf.Reset()
	_, err = row.WriteTo(&r.buf)
	r.pos++
	return err
}

func (r *rowStreamReader) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	if r.err != nil {
		return 0, r.err
	}
	if r.buf.Len() == 0 {
		if r.pos == r.rows {
			return 0, io.EOF
		}
		if r.err = r.load(); r.err != nil {
			return 0, r.err
		}
	}
	return r.buf.Read(p)
}
