package bitswap

import (
	"context"
	"errors"
	"fmt"

	bstore "github.com/ipfs/boxo/blockstore"
	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"
	ipld "github.com/ipfs/go-ipld-format"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

const (
	notFoundKey = "not_found"
	failedKey   = "failed"
)

var (
	meter                   = otel.Meter("bitswap_blockstore")
	_     bstore.Blockstore = (*BlockstoreWithMetrics)(nil)
)

// BlockstoreWithMetrics is a blockstore that collects metrics on blockstore operations.
type BlockstoreWithMetrics struct {
	bs      bstore.Blockstore
	metrics *metrics
}

type metrics struct {
	delete      metric.Int64Counter
	has         metric.Int64Counter
	get         metric.Int64Counter
	getSize     metric.Int64Counter
	put         metric.Int64Counter
	putMany     metric.Int64Counter
	allKeysChan metric.Int64Counter
}

// NewBlockstoreWithMetrics creates a new BlockstoreWithMetrics.
func NewBlockstoreWithMetrics(bs bstore.Blockstore) (*BlockstoreWithMetrics, error) {
	return &BlockstoreWithMetrics{
		bs: bs,
	}, nil
}

// WithMetrics enables metrics collection on the blockstore.
func (w *BlockstoreWithMetrics) WithMetrics() error {
	del, err := meter.Int64Counter(
		"blockstore_delete_block",
		metric.WithDescription("Blockstore delete block operation"),
	)
	if err != nil {
		return fmt.Errorf("failed to create blockstore delete block counter: %w", err)
	}

	has, err := meter.Int64Counter(
		"blockstore_has",
		metric.WithDescription("Blockstore has operation"),
	)
	if err != nil {
		return fmt.Errorf("failed to create blockstore has counter: %w", err)
	}

	get, err := meter.Int64Counter(
		"blockstore_get",
		metric.WithDescription("Blockstore get operation"),
	)
	if err != nil {
		return fmt.Errorf("failed to create blockstore get counter: %w", err)
	}

	getSize, err := meter.Int64Counter(
		"blockstore_get_size",
		metric.WithDescription("Blockstore get size operation"),
	)
	if err != nil {
		return fmt.Errorf("failed to create blockstore get size counter: %w", err)
	}

	put, err := meter.Int64Counter(
		"blockstore_put",
		metric.WithDescription("Blockstore put operation"),
	)
	if err != nil {
		return fmt.Errorf("failed to create blockstore put counter: %w", err)
	}

	putMany, err := meter.Int64Counter(
		"blockstore_put_many",
		metric.WithDescription("Blockstore put many operation"),
	)
	if err != nil {
		return fmt.Errorf("failed to create blockstore put many counter: %w", err)
	}

	allKeysChan, err := meter.Int64Counter(
		"blockstore_all_keys_chan",
		metric.WithDescription("Blockstore all keys chan operation"),
	)
	if err != nil {
		return fmt.Errorf("failed to create blockstore all keys chan counter: %w", err)
	}

	if err != nil {
		return fmt.Errorf("failed to create blockstore hash on read counter: %w", err)
	}

	w.metrics = &metrics{
		delete:      del,
		has:         has,
		get:         get,
		getSize:     getSize,
		put:         put,
		putMany:     putMany,
		allKeysChan: allKeysChan,
	}
	return nil
}

func (w *BlockstoreWithMetrics) DeleteBlock(ctx context.Context, cid cid.Cid) error {
	if w.metrics == nil {
		return w.bs.DeleteBlock(ctx, cid)
	}
	err := w.bs.DeleteBlock(ctx, cid)
	w.metrics.delete.Add(ctx, 1, metric.WithAttributes(
		attribute.Bool(failedKey, err != nil),
	))
	return err
}

func (w *BlockstoreWithMetrics) Has(ctx context.Context, cid cid.Cid) (bool, error) {
	if w.metrics == nil {
		return w.bs.Has(ctx, cid)
	}
	has, err := w.bs.Has(ctx, cid)
	notFound := errors.Is(err, ipld.ErrNotFound{})
	failed := err != nil && !notFound
	w.metrics.has.Add(ctx, 1, metric.WithAttributes(
		attribute.Bool(failedKey, failed),
		attribute.Bool(notFoundKey, notFound),
	))
	return has, err
}

func (w *BlockstoreWithMetrics) Get(ctx context.Context, cid cid.Cid) (blocks.Block, error) {
	if w.metrics == nil {
		return w.bs.Get(ctx, cid)
	}
	get, err := w.bs.Get(ctx, cid)
	notFound := errors.Is(err, ipld.ErrNotFound{})
	failed := err != nil && !notFound
	w.metrics.get.Add(ctx, 1, metric.WithAttributes(
		attribute.Bool(failedKey, failed),
		attribute.Bool(notFoundKey, notFound),
	))
	return get, err
}

func (w *BlockstoreWithMetrics) GetSize(ctx context.Context, cid cid.Cid) (int, error) {
	if w.metrics == nil {
		return w.bs.GetSize(ctx, cid)
	}
	size, err := w.bs.GetSize(ctx, cid)
	notFound := errors.Is(err, ipld.ErrNotFound{})
	failed := err != nil && !notFound
	w.metrics.getSize.Add(ctx, 1, metric.WithAttributes(
		attribute.Bool(failedKey, failed),
		attribute.Bool(notFoundKey, notFound),
	))
	return size, err
}

func (w *BlockstoreWithMetrics) Put(ctx context.Context, block blocks.Block) error {
	if w.metrics == nil {
		return w.bs.Put(ctx, block)
	}
	err := w.bs.Put(ctx, block)
	w.metrics.put.Add(ctx, 1, metric.WithAttributes(
		attribute.Bool(failedKey, err != nil),
	))
	return err
}

func (w *BlockstoreWithMetrics) PutMany(ctx context.Context, blocks []blocks.Block) error {
	if w.metrics == nil {
		return w.bs.PutMany(ctx, blocks)
	}
	err := w.bs.PutMany(ctx, blocks)
	w.metrics.putMany.Add(ctx, 1, metric.WithAttributes(
		attribute.Bool(failedKey, err != nil),
	))
	return err
}

func (w *BlockstoreWithMetrics) AllKeysChan(ctx context.Context) (<-chan cid.Cid, error) {
	if w.metrics == nil {
		return w.bs.AllKeysChan(ctx)
	}
	ch, err := w.bs.AllKeysChan(ctx)
	w.metrics.allKeysChan.Add(ctx, 1, metric.WithAttributes(
		attribute.Bool(failedKey, err != nil),
	))
	return ch, err
}

func (w *BlockstoreWithMetrics) HashOnRead(enabled bool) {
	w.bs.HashOnRead(enabled)
}
