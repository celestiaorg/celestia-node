package cache

import (
	"context"
	"io"

	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/share"
	eds "github.com/celestiaorg/celestia-node/share/new_eds"
	"github.com/celestiaorg/celestia-node/share/shwap"
)

var _ Cache = (*NoopCache)(nil)

// NoopCache implements noop version of Cache interface
type NoopCache struct{}

func (n NoopCache) Get(uint64) (eds.AccessorStreamer, error) {
	return nil, ErrCacheMiss
}

func (n NoopCache) GetOrLoad(ctx context.Context, _ uint64, loader OpenAccessorFn) (eds.AccessorStreamer, error) {
	return loader(ctx)
}

func (n NoopCache) Remove(uint64) error {
	return nil
}

func (n NoopCache) EnableMetrics() error {
	return nil
}

var _ eds.AccessorStreamer = NoopFile{}

// NoopFile implements noop version of eds.AccessorStreamer interface
type NoopFile struct{}

func (n NoopFile) Reader() (io.Reader, error) {
	return noopReader{}, nil
}

func (n NoopFile) Size(context.Context) int {
	return 0
}

func (n NoopFile) Sample(context.Context, int, int) (shwap.Sample, error) {
	return shwap.Sample{}, nil
}

func (n NoopFile) AxisHalf(context.Context, rsmt2d.Axis, int) (eds.AxisHalf, error) {
	return eds.AxisHalf{}, nil
}

func (n NoopFile) RowNamespaceData(context.Context, share.Namespace, int) (shwap.RowNamespaceData, error) {
	return shwap.RowNamespaceData{}, nil
}

func (n NoopFile) Shares(context.Context) ([]share.Share, error) {
	return []share.Share{}, nil
}

func (n NoopFile) Close() error {
	return nil
}

type noopReader struct{}

func (n noopReader) Read([]byte) (int, error) {
	return 0, nil
}
