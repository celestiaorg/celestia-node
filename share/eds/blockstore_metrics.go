package eds

import (
	"context"
	"errors"
	"fmt"

	bstore "github.com/ipfs/boxo/blockstore"
	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"
	ipld "github.com/ipfs/go-ipld-format"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

const (
	notFoundKey = "not_found"
)

var _ bstore.Blockstore = (*withMetrics)(nil)

type withMetrics struct {
	bs bstore.Blockstore

	delete      metric.Int64Counter
	has         metric.Int64Counter
	get         metric.Int64Counter
	getSize     metric.Int64Counter
	put         metric.Int64Counter
	putMany     metric.Int64Counter
	allKeysChan metric.Int64Counter
	hashOnRead  metric.Int64Counter
}

func NewBlockstoreWithMetrics(bs bstore.Blockstore) (*withMetrics, error) {
	delete, err := meter.Int64Counter(
		"blockstore_delete_block",
		metric.WithDescription("Blockstore delete block operation"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create blockstore delete block counter: %w", err)
	}

	has, err := meter.Int64Counter(
		"blockstore_has",
		metric.WithDescription("Blockstore has operation"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create blockstore has counter: %w", err)
	}

	get, err := meter.Int64Counter(
		"blockstore_get",
		metric.WithDescription("Blockstore get operation"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create blockstore get counter: %w", err)
	}

	getSize, err := meter.Int64Counter(
		"blockstore_get_size",
		metric.WithDescription("Blockstore get size operation"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create blockstore get size counter: %w", err)
	}

	put, err := meter.Int64Counter(
		"blockstore_put",
		metric.WithDescription("Blockstore put operation"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create blockstore put counter: %w", err)
	}

	putMany, err := meter.Int64Counter(
		"blockstore_put_many",
		metric.WithDescription("Blockstore put many operation"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create blockstore put many counter: %w", err)
	}

	allKeysChan, err := meter.Int64Counter(
		"blockstore_all_keys_chan",
		metric.WithDescription("Blockstore all keys chan operation"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create blockstore all keys chan counter: %w", err)
	}

	hashOnRead, err := meter.Int64Counter(
		"blockstore_hash_on_read",
		metric.WithDescription("Blockstore hash on read operation"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create blockstore hash on read counter: %w", err)
	}

	return &withMetrics{
		bs:          bs,
		delete:      delete,
		has:         has,
		get:         get,
		getSize:     getSize,
		put:         put,
		putMany:     putMany,
		allKeysChan: allKeysChan,
		hashOnRead:  hashOnRead,
	}, nil
}

func (w *withMetrics) DeleteBlock(ctx context.Context, cid cid.Cid) error {
	// TODO implement me
	panic("implement me")
}

func (w *withMetrics) Has(ctx context.Context, cid cid.Cid) (bool, error) {
	has, err := w.bs.Has(ctx, cid)
	notFound := errors.Is(err, ipld.ErrNotFound{})
	failed := err != nil && !notFound
	w.has.Add(ctx, 1, metric.WithAttributes(
		attribute.Bool(failedKey, failed),
		attribute.Bool(notFoundKey, notFound),
	))
	return has, err
}

func (w *withMetrics) Get(ctx context.Context, cid cid.Cid) (blocks.Block, error) {
	get, err := w.bs.Get(ctx, cid)
	notFound := errors.Is(err, ipld.ErrNotFound{})
	failed := err != nil && !notFound
	w.get.Add(ctx, 1, metric.WithAttributes(
		attribute.Bool(failedKey, failed),
		attribute.Bool(notFoundKey, notFound),
	))
	return get, err
}

func (w *withMetrics) GetSize(ctx context.Context, cid cid.Cid) (int, error) {
	size, err := w.bs.GetSize(ctx, cid)
	notFound := errors.Is(err, ipld.ErrNotFound{})
	failed := err != nil && !notFound
	w.getSize.Add(ctx, 1, metric.WithAttributes(
		attribute.Bool(failedKey, failed),
		attribute.Bool(notFoundKey, notFound),
	))
	return size, err
}

func (w *withMetrics) Put(ctx context.Context, block blocks.Block) error {
	err := w.bs.Put(ctx, block)
	w.put.Add(ctx, 1, metric.WithAttributes(
		attribute.Bool(failedKey, err != nil),
	))
	return err
}

func (w *withMetrics) PutMany(ctx context.Context, blocks []blocks.Block) error {
	err := w.bs.PutMany(ctx, blocks)
	w.putMany.Add(ctx, 1, metric.WithAttributes(
		attribute.Bool(failedKey, err != nil),
	))
	return err
}

func (w *withMetrics) AllKeysChan(ctx context.Context) (<-chan cid.Cid, error) {
	ch, err := w.bs.AllKeysChan(ctx)
	w.allKeysChan.Add(ctx, 1, metric.WithAttributes(
		attribute.Bool(failedKey, err != nil),
	))
	return ch, err
}

func (w *withMetrics) HashOnRead(enabled bool) {
	// TODO implement me
	panic("implement me")
}
