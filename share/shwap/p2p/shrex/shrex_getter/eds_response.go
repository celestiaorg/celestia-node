package shrex_getter

import (
	"context"
	"io"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds"
)

// edsResponse adapts eds.ReadAccessor to the shrex response
type edsResponse struct {
	ctx  context.Context
	root *share.AxisRoots
	eds  *eds.Rsmt2D
}

func (r *edsResponse) ReadFrom(src io.Reader) (int64, error) {
	cr := &countingReader{r: src}
	accessor, err := eds.ReadAccessor(r.ctx, cr, r.root)
	if err != nil {
		return cr.n, err
	}
	r.eds = accessor
	return cr.n, nil
}

// countingReader tracks bytes read so ReadFrom can report the transferred size
type countingReader struct {
	r io.Reader
	n int64
}

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.n += int64(n)
	return n, err
}
