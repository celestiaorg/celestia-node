package cache

import (
	"context"
	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/store/file"
	"github.com/celestiaorg/rsmt2d"
	"io"
)

var _ Cache = (*NoopCache)(nil)

// NoopCache implements noop version of Cache interface
type NoopCache struct{}

func (n NoopCache) Get(Key) (file.EdsFile, error) {
	return nil, errCacheMiss
}

func (n NoopCache) GetOrLoad(context.Context, Key, OpenFileFn) (file.EdsFile, error) {
	return NoopFile{}, nil
}

func (n NoopCache) Remove(Key) error {
	return nil
}

func (n NoopCache) EnableMetrics() error {
	return nil
}

var _ file.EdsFile = (*NoopFile)(nil)

// NoopFile implements noop version of file.EdsFile interface
type NoopFile struct{}

func (n NoopFile) Close() error {
	return nil
}

func (n NoopFile) Reader() (io.Reader, error) {
	return nil, nil
}

func (n NoopFile) Size() int {
	return 0
}

func (n NoopFile) Height() uint64 {
	return 0
}

func (n NoopFile) DataHash() share.DataHash {
	return nil
}

func (n NoopFile) Share(ctx context.Context, x, y int) (*share.ShareWithProof, error) {
	return nil, nil
}

func (n NoopFile) AxisHalf(ctx context.Context, axisType rsmt2d.Axis, axisIdx int) ([]share.Share, error) {
	return nil, nil
}

func (n NoopFile) Data(ctx context.Context, namespace share.Namespace, rowIdx int) (share.NamespacedRow, error) {
	return share.NamespacedRow{}, nil
}

func (n NoopFile) EDS(ctx context.Context) (*rsmt2d.ExtendedDataSquare, error) {
	return nil, nil
}
