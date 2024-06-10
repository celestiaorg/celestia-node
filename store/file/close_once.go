package file

import (
	"context"
	"errors"
	"sync/atomic"

	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/share"
	eds "github.com/celestiaorg/celestia-node/share/new_eds"
	"github.com/celestiaorg/celestia-node/share/shwap"
)

var _ eds.AccessorCloser = (*closeOnce)(nil)

var errAccessorClosed = errors.New("accessor is closed")

type closeOnce struct {
	f      eds.AccessorCloser
	closed atomic.Bool
}

func WithClosedOnce(f eds.AccessorCloser) eds.AccessorCloser {
	return &closeOnce{f: f}
}

func (c *closeOnce) Close() error {
	if !c.closed.Swap(true) {
		err := c.f.Close()
		// release reference to the accessor to allow GC to collect all resources associated with it
		c.f = nil
		return err
	}
	return nil
}

func (c *closeOnce) Size(ctx context.Context) int {
	if c.closed.Load() {
		return 0
	}
	return c.f.Size(ctx)
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
) (eds.AxisHalf, error) {
	if c.closed.Load() {
		return eds.AxisHalf{}, errAccessorClosed
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
