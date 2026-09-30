package eds

import (
	"context"
	"errors"
	"io"
	"sync/atomic"

	libshare "github.com/celestiaorg/go-square/v4/share"
	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/shwap"
)

var _ AccessorStreamer = (*closeOnce)(nil)

var errAccessorClosed = errors.New("accessor is closed")

type closeOnce struct {
	// f is swapped to nil on Close, so methods racing with Close either see the accessor or
	// errAccessorClosed, never a half-cleared field.
	f atomic.Pointer[AccessorStreamer]
}

func WithClosedOnce(f AccessorStreamer) AccessorStreamer {
	c := &closeOnce{}
	c.f.Store(&f)
	return c
}

func (c *closeOnce) Close() error {
	// release reference to the accessor to allow GC to collect all resources associated with it
	f := c.f.Swap(nil)
	if f == nil {
		return nil
	}
	return (*f).Close()
}

func (c *closeOnce) accessor() (AccessorStreamer, error) {
	f := c.f.Load()
	if f == nil {
		return nil, errAccessorClosed
	}
	return *f, nil
}

func (c *closeOnce) Size(ctx context.Context) (int, error) {
	f, err := c.accessor()
	if err != nil {
		return 0, err
	}
	return f.Size(ctx)
}

func (c *closeOnce) DataHash(ctx context.Context) (share.DataHash, error) {
	f, err := c.accessor()
	if err != nil {
		return nil, err
	}
	return f.DataHash(ctx)
}

func (c *closeOnce) AxisRoots(ctx context.Context) (*share.AxisRoots, error) {
	f, err := c.accessor()
	if err != nil {
		return nil, err
	}
	return f.AxisRoots(ctx)
}

func (c *closeOnce) Sample(ctx context.Context, idx shwap.SampleCoords) (shwap.Sample, error) {
	f, err := c.accessor()
	if err != nil {
		return shwap.Sample{}, err
	}
	return f.Sample(ctx, idx)
}

func (c *closeOnce) AxisHalf(
	ctx context.Context,
	axisType rsmt2d.Axis,
	axisIdx int,
) (shwap.AxisHalf, error) {
	f, err := c.accessor()
	if err != nil {
		return shwap.AxisHalf{}, err
	}
	return f.AxisHalf(ctx, axisType, axisIdx)
}

func (c *closeOnce) RowNamespaceData(
	ctx context.Context,
	namespace libshare.Namespace,
	rowIdx int,
) (shwap.RowNamespaceData, error) {
	f, err := c.accessor()
	if err != nil {
		return shwap.RowNamespaceData{}, err
	}
	return f.RowNamespaceData(ctx, namespace, rowIdx)
}

func (c *closeOnce) Shares(ctx context.Context) ([]libshare.Share, error) {
	f, err := c.accessor()
	if err != nil {
		return nil, err
	}
	return f.Shares(ctx)
}

func (c *closeOnce) RangeNamespaceData(
	ctx context.Context,
	from, to int,
) (shwap.RangeNamespaceData, error) {
	f, err := c.accessor()
	if err != nil {
		return shwap.RangeNamespaceData{}, err
	}
	return f.RangeNamespaceData(ctx, from, to)
}

func (c *closeOnce) Reader() (io.Reader, error) {
	f, err := c.accessor()
	if err != nil {
		return nil, err
	}
	return f.Reader()
}
