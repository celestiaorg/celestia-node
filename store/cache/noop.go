package cache

import (
	"context"
	"io"

	libshare "github.com/celestiaorg/go-square/v2/share"
	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds"
	"github.com/celestiaorg/celestia-node/share/shwap"
)

var _ Cache = (*NoopCache)(nil)

// NoopCache implements noop version of Cache interface
type NoopCache struct{}

func (n NoopCache) Has(uint64) bool {
	return false
}

func (n NoopCache) Get(uint64) (eds.AccessorStreamer, error) {
	return nil, ErrCacheMiss
}

func (n NoopCache) GetOrLoad(ctx context.Context, _ uint64, loader OpenAccessorFn) (eds.AccessorStreamer, error) {
	return loader(ctx)
}

func (n NoopCache) Remove(uint64) error {
	return nil
}

func (n NoopCache) EnableMetrics() (unreg func() error, err error) {
	noop := func() error { return nil }
	return noop, nil
}

var _ eds.AccessorStreamer = NoopFile{}

// NoopFile implements noop version of eds.AccessorStreamer interface
type NoopFile struct{}

func (n NoopFile) Reader() (io.Reader, error) {
	return noopReader{}, nil
}

func (n NoopFile) Size(context.Context) (int, error) {
	return 0, nil
}

func (n NoopFile) DataHash(context.Context) (share.DataHash, error) {
	return share.DataHash{}, nil
}

func (n NoopFile) AxisRoots(context.Context) (*share.AxisRoots, error) {
	return &share.AxisRoots{}, nil
}

func (n NoopFile) Sample(context.Context, shwap.SampleCoords) (shwap.Sample, error) {
	return shwap.Sample{}, nil
}

func (n NoopFile) AxisHalf(context.Context, rsmt2d.Axis, int) (shwap.AxisHalf, error) {
	return shwap.AxisHalf{}, nil
}

func (n NoopFile) RowNamespaceData(context.Context, libshare.Namespace, int) (shwap.RowNamespaceData, error) {
	return shwap.RowNamespaceData{}, nil
}

func (n NoopFile) RangeNamespaceData(
	_ context.Context,
	_, _ int,
) (shwap.RangeNamespaceData, error) {
	return shwap.RangeNamespaceData{}, nil
}

func (n NoopFile) Shares(context.Context) ([]libshare.Share, error) {
	return []libshare.Share{}, nil
}

func (n NoopFile) Close() error {
	return nil
}

type noopReader struct{}

func (n noopReader) Read([]byte) (int, error) {
	return 0, nil
}
