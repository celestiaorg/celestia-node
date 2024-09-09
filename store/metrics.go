package store

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"github.com/celestiaorg/celestia-node/store/cache"
)

const (
	failedKey = "failed"
	sizeKey   = "eds_size"
)

var meter = otel.Meter("store")

type metrics struct {
	put       metric.Float64Histogram
	putExists metric.Int64Counter
	get       metric.Float64Histogram
	has       metric.Float64Histogram
	removeAll metric.Float64Histogram
	removeQ4  metric.Float64Histogram
	unreg     func() error
}

func (s *Store) WithMetrics() error {
	put, err := meter.Float64Histogram("eds_store_put_time_histogram",
		metric.WithDescription("eds store put time histogram(s)"))
	if err != nil {
		return err
	}

	putExists, err := meter.Int64Counter("eds_store_put_exists_counter",
		metric.WithDescription("eds store put file exists"))
	if err != nil {
		return err
	}

	get, err := meter.Float64Histogram("eds_store_get_time_histogram",
		metric.WithDescription("eds store get time histogram(s)"))
	if err != nil {
		return err
	}

	has, err := meter.Float64Histogram("eds_store_has_time_histogram",
		metric.WithDescription("eds store has time histogram(s)"))
	if err != nil {
		return err
	}

	removeQ4, err := meter.Float64Histogram("eds_store_remove_q4_time_histogram",
		metric.WithDescription("eds store remove q4 data time histogram(s)"))
	if err != nil {
		return err
	}

	removeAll, err := meter.Float64Histogram("eds_store_remove_all_time_histogram",
		metric.WithDescription("eds store remove all data time histogram(s)"))
	if err != nil {
		return err
	}

	s.metrics = &metrics{
		put:       put,
		putExists: putExists,
		get:       get,
		has:       has,
		removeAll: removeAll,
		removeQ4:  removeQ4,
	}
	return s.metrics.addCacheMetrics(s.cache)
}

// addCacheMetrics adds cache metrics to store metrics
func (m *metrics) addCacheMetrics(c cache.Cache) error {
	if m == nil {
		return nil
	}
	unreg, err := c.EnableMetrics()
	if err != nil {
		return fmt.Errorf("while enabling metrics for cache: %w", err)
	}
	m.unreg = unreg
	return nil
}

func (m *metrics) observePut(ctx context.Context, dur time.Duration, size uint, failed bool) {
	if m == nil {
		return
	}
	if ctx.Err() != nil {
		ctx = context.Background()
	}

	m.put.Record(ctx, dur.Seconds(), metric.WithAttributes(
		attribute.Bool(failedKey, failed),
		attribute.Int(sizeKey, int(size))))
}

func (m *metrics) observePutExist(ctx context.Context) {
	if m == nil {
		return
	}
	if ctx.Err() != nil {
		ctx = context.Background()
	}

	m.putExists.Add(ctx, 1)
}

func (m *metrics) observeGet(ctx context.Context, dur time.Duration, failed bool) {
	if m == nil {
		return
	}
	if ctx.Err() != nil {
		ctx = context.Background()
	}

	m.get.Record(ctx, dur.Seconds(), metric.WithAttributes(
		attribute.Bool(failedKey, failed)))
}

func (m *metrics) observeHas(ctx context.Context, dur time.Duration, failed bool) {
	if m == nil {
		return
	}
	if ctx.Err() != nil {
		ctx = context.Background()
	}

	m.has.Record(ctx, dur.Seconds(), metric.WithAttributes(
		attribute.Bool(failedKey, failed)))
}

func (m *metrics) observeRemoveAll(ctx context.Context, dur time.Duration, failed bool) {
	if m == nil {
		return
	}
	if ctx.Err() != nil {
		ctx = context.Background()
	}

	m.removeAll.Record(ctx, dur.Seconds(), metric.WithAttributes(
		attribute.Bool(failedKey, failed)))
}

func (m *metrics) observeRemoveQ4(ctx context.Context, dur time.Duration, failed bool) {
	if m == nil {
		return
	}
	if ctx.Err() != nil {
		ctx = context.Background()
	}

	m.removeQ4.Record(ctx, dur.Seconds(), metric.WithAttributes(
		attribute.Bool(failedKey, failed)))
}

func (m *metrics) close() error {
	if m == nil {
		return nil
	}
	return m.unreg()
}
