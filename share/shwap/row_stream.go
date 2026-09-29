package shwap

import (
	"context"
	"fmt"
	"io"
	"math"
	"sync"

	libshare "github.com/celestiaorg/go-square/v4/share"
	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/share"
)

// rowStreamWindow is how many rows are fetched ahead of the writer.
const rowStreamWindow = 4

// rowStreamReserve is the memory a row-by-row streamed response holds, whatever the square size:
// rowStreamWindow rows in flight, each held in extended and in encoded form, plus the row being
// written out and the first row of a range, which is kept for the whole stream.
func rowStreamReserve(edsSize int) int {
	proofSize := 2 * share.AxisRootSize * int(math.Log2(float64(edsSize)))
	rowSize := edsSize*libshare.ShareSize + proofSize
	return (2*rowStreamWindow + 2) * rowSize
}

func extendedRow(ctx context.Context, acc Accessor, rowIdx int) ([]libshare.Share, error) {
	half, err := acc.AxisHalf(ctx, rsmt2d.Row, rowIdx)
	if err != nil {
		return nil, fmt.Errorf("getting row %d: %w", rowIdx, err)
	}
	shares, err := half.Extended()
	if err != nil {
		return nil, fmt.Errorf("extending row %d: %w", rowIdx, err)
	}
	return shares, nil
}

type encodedRow struct {
	data []byte
	err  error
}

// rowStreamReader emits encoded rows in order while fetching up to rowStreamWindow of them
// concurrently, so a response is never held whole in memory. It implements io.Closer and must be
// closed before the accessor it reads from.
type rowStreamReader struct {
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
	window chan struct{}
	rows   []chan encodedRow
	pos    int
	buf    []byte
	err    error
}

// newRowStreamReader starts the fetchers and waits for the first row, so that accessor errors
// surface before the server commits to an OK response.
func newRowStreamReader(
	ctx context.Context,
	rows int,
	fetch func(ctx context.Context, row int) ([]byte, error),
) (*rowStreamReader, error) {
	ctx, cancel := context.WithCancel(ctx)
	r := &rowStreamReader{
		ctx:    ctx,
		cancel: cancel,
		window: make(chan struct{}, rowStreamWindow),
		rows:   make([]chan encodedRow, rows),
	}
	for i := range r.rows {
		r.rows[i] = make(chan encodedRow, 1)
	}

	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		for i := range rows {
			// a slot is freed once the row is consumed by next.
			select {
			case r.window <- struct{}{}:
			case <-ctx.Done():
				return
			}
			r.wg.Add(1)
			go func() {
				defer r.wg.Done()
				data, err := fetch(ctx, i)
				r.rows[i] <- encodedRow{data: data, err: err}
			}()
		}
	}()

	if rows > 0 {
		if err := r.next(); err != nil {
			r.Close()
			return nil, err
		}
	}
	return r, nil
}

func (r *rowStreamReader) Read(p []byte) (int, error) {
	if len(r.buf) == 0 {
		switch {
		case r.err != nil:
			return 0, r.err
		case r.pos == len(r.rows):
			return 0, io.EOF
		}
		if err := r.next(); err != nil {
			return 0, err
		}
	}
	n := copy(p, r.buf)
	r.buf = r.buf[n:]
	return n, nil
}

// The error next returns is sticky: a stream that failed mid-way must not silently resume on the
// row after the failed one.
func (r *rowStreamReader) next() error {
	var row encodedRow
	select {
	case row = <-r.rows[r.pos]:
	case <-r.ctx.Done():
		r.err = r.ctx.Err()
		return r.err
	}
	r.rows[r.pos] = nil
	r.pos++
	<-r.window

	if row.err != nil {
		r.err = row.err
		return r.err
	}
	r.buf = row.data
	return nil
}

// Close stops outstanding fetches and waits for them to finish.
func (r *rowStreamReader) Close() error {
	r.cancel()
	r.wg.Wait()
	return nil
}
