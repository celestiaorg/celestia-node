package shrex_getter

import (
	"bytes"
	"context"
	"fmt"
	"io"

	libshare "github.com/celestiaorg/go-square/v4/share"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds"
)

// edsResponse streams the raw ODS shares off the shrex stream. Reconstruction and hash
// verification are deferred to verify (run in the build step) so the libp2p stream isn't held
// open during that CPU-bound work.
type edsResponse struct {
	odsSize int
	shares  []libshare.Share
	eds     *eds.Rsmt2D
}

func (r *edsResponse) ReadFrom(src io.Reader) (int64, error) {
	cr := &countingReader{r: src}
	shares, err := eds.ReadShares(cr, libshare.ShareSize, r.odsSize)
	if err != nil {
		return cr.n, err
	}
	r.shares = shares
	return cr.n, nil
}

func (r *edsResponse) verify(ctx context.Context, root *share.AxisRoots) error {
	square, err := eds.Rsmt2DFromShares(r.shares, r.odsSize)
	if err != nil {
		return err
	}
	datahash, err := square.DataHash(ctx)
	if err != nil {
		return err
	}
	if !bytes.Equal(datahash, root.Hash()) {
		return fmt.Errorf(
			"content integrity mismatch: imported root %s doesn't match expected root %s",
			datahash, root.Hash(),
		)
	}
	r.eds = square
	return nil
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
