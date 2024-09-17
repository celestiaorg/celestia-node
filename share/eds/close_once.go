package eds

import (
	"context"
	"errors"
	"io"
	"sync/atomic"

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

func (c *closeOnce) Size(ctx context.Context) int {
	if c.closed.Load() {
		return 0
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

func (c *closeOnce) Sample(ctx context.Context, rowIdx, colIdx int) (shwap.Sample, error) {
	if c.closed.Load() {
		return shwap.Sample{}, errAccessorClosed
	}
	return c.f.Sample(ctx, rowIdx, colIdx)
}

func (c *closeOnce) AxisHalf(
	ctx context.Context,
	axisType rsmt2d.Axis,
	axisIdx int,
) (AxisHalf, error) {
	if c.closed.Load() {
		return AxisHalf{}, errAccessorClosed
	}
	return c.f.AxisHalf(ctx, axisType, axisIdx)
}

func (c *closeOnce) RowNamespaceData(
	ctx context.Context,
	namespace share.Namespace,
	rowIdx int,
) (shwap.RowNamespaceData, error) {
	if c.closed.Load() {
		return shwap.RowNamespaceData{}, errAccessorClosed
	}
	return c.f.RowNamespaceData(ctx, namespace, rowIdx)
}

func (c *closeOnce) Shares(ctx context.Context) ([]share.Share, error) {
	if c.closed.Load() {
		return nil, errAccessorClosed
	}
	return c.f.Shares(ctx)
}

func (c *closeOnce) Reader() (io.Reader, error) {
	if c.closed.Load() {
		return nil, errAccessorClosed
	}
	return c.f.Reader()
}
