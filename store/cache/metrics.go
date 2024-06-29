package cache

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

const (
	cacheFoundKey = "found"
	failedKey     = "failed"
)

type metrics struct {
	getCounter     metric.Int64Counter
	evictedCounter metric.Int64Counter
}

func newMetrics(bc *AccessorCache) (*metrics, error) {
	metricsPrefix := "eds_blockstore_cache_" + bc.name

	evictedCounter, err := meter.Int64Counter(metricsPrefix+"_evicted_counter",
		metric.WithDescription("eds blockstore cache evicted event counter"))
	if err != nil {
		return nil, err
	}

	getCounter, err := meter.Int64Counter(metricsPrefix+"_get_counter",
		metric.WithDescription("eds blockstore cache evicted event counter"))
	if err != nil {
		return nil, err
	}

	cacheSize, err := meter.Int64ObservableGauge(metricsPrefix+"_size",
		metric.WithDescription("total amount of items in blockstore cache"),
	)
	if err != nil {
		return nil, err
	}

	callback := func(_ context.Context, observer metric.Observer) error {
		observer.ObserveInt64(cacheSize, int64(bc.cache.Len()))
		return nil
	}
	_, err = meter.RegisterCallback(callback, cacheSize)

	return &metrics{
		getCounter:     getCounter,
		evictedCounter: evictedCounter,
	}, err
}

func (m *metrics) observeEvicted(failed bool) {
	if m == nil {
		return
	}
	m.evictedCounter.Add(context.Background(), 1,
		metric.WithAttributes(
			attribute.Bool(failedKey, failed)))
}

func (m *metrics) observeGet(found bool) {
	if m == nil {
		return
	}
	m.getCounter.Add(context.Background(), 1, metric.WithAttributes(
		attribute.Bool(cacheFoundKey, found)))
}
