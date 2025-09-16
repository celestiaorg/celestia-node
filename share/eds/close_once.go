package eds

import (
	"context"
	"errors"
	"io"
	"sync/atomic"

	libshare "github.com/celestiaorg/go-square/v2/share"
	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/shwap"
)

var _ AccessorStreamer = (*closeOnce)(nil)

var errAccessorClosed = errors.New("accessor is closed")

type closeOnce struct {
	f      AccessorStreamer
	closed atomic.Bool
}

func WithClosedOnce(f AccessorStreamer) AccessorStreamer {
	return &closeOnce{f: f}
}

func (c *closeOnce) Close() error {
	if c.closed.Swap(true) {
		return nil
	}
	err := c.f.Close()
	// release reference to the accessor to allow GC to collect all resources associated with it
	c.f = nil
	return err
}

func (c *closeOnce) Size(ctx context.Context) (int, error) {
	if c.closed.Load() {
		return 0, errAccessorClosed
	}
	return c.f.Size(ctx)
}

func (c *closeOnce) DataHash(ctx context.Context) (share.DataHash, error) {
	if c.closed.Load() {
		return nil, errAccessorClosed
	}
	return c.f.DataHash(ctx)
}

func (c *closeOnce) AxisRoots(ctx context.Context) (*share.AxisRoots, error) {
	if c.closed.Load() {
		return nil, errAccessorClosed
	}
	return c.f.AxisRoots(ctx)
}

func (c *closeOnce) Sample(ctx context.Context, idx shwap.SampleCoords) (shwap.Sample, error) {
	if c.closed.Load() {
		return shwap.Sample{}, errAccessorClosed
	}
	return c.f.Sample(ctx, idx)
}

func (c *closeOnce) AxisHalf(
	ctx context.Context,
	axisType rsmt2d.Axis,
	axisIdx int,
) (shwap.AxisHalf, error) {
	if c.closed.Load() {
		return shwap.AxisHalf{}, errAccessorClosed
	}
	return c.f.AxisHalf(ctx, axisType, axisIdx)
}

func (c *closeOnce) RowNamespaceData(
	ctx context.Context,
	namespace libshare.Namespace,
	rowIdx int,
) (shwap.RowNamespaceData, error) {
	if c.closed.Load() {
		return shwap.RowNamespaceData{}, errAccessorClosed
	}
	return c.f.RowNamespaceData(ctx, namespace, rowIdx)
}

func (c *closeOnce) Shares(ctx context.Context) ([]libshare.Share, error) {
	if c.closed.Load() {
		return nil, errAccessorClosed
	}
	return c.f.Shares(ctx)
}

func (c *closeOnce) RangeNamespaceData(
	ctx context.Context,
	from, to int,
) (shwap.RangeNamespaceData, error) {
	if c.closed.Load() {
		return shwap.RangeNamespaceData{}, errAccessorClosed
	}
	return c.f.RangeNamespaceData(ctx, from, to)
}

func (c *closeOnce) Reader() (io.Reader, error) {
	if c.closed.Load() {
		return nil, errAccessorClosed
	}
	return c.f.Reader()
}
