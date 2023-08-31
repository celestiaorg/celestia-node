package eds

import (
	"context"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

const (
	failedKey = "failed"
	sizeKey   = "eds_size"

	putResultKey           = "result"
	putOK        putResult = "ok"
	putExists    putResult = "exists"
	putFailed    putResult = "failed"

	opNameKey                     = "op"
	longOpResultKey               = "result"
	longOpUnresolved longOpResult = "unresolved"
	longOpOK         longOpResult = "ok"
	longOpFailed     longOpResult = "failed"
)

var (
	meter = otel.Meter("eds_store")
)

type putResult string

type longOpResult string

type metrics struct {
	putTime              metric.Float64Histogram
	getCARTime           metric.Float64Histogram
	getCARBlockstoreTime metric.Float64Histogram
	getDAHTime           metric.Float64Histogram
	removeTime           metric.Float64Histogram
	getTime              metric.Float64Histogram
	hasTime              metric.Float64Histogram
	listTime             metric.Float64Histogram

	longOpTime metric.Float64Histogram
	gcTime     metric.Float64Histogram
}

func (s *Store) WithMetrics() error {
	putTime, err := meter.Float64Histogram("eds_store_put_time_histogram",
		metric.WithDescription("eds store put time histogram(s)"))
	if err != nil {
		return err
	}

	getCARTime, err := meter.Float64Histogram("eds_store_get_car_time_histogram",
		metric.WithDescription("eds store get car time histogram(s)"))
	if err != nil {
		return err
	}

	getCARBlockstoreTime, err := meter.Float64Histogram("eds_store_get_car_blockstore_time_histogram",
		metric.WithDescription("eds store get car blockstore time histogram(s)"))
	if err != nil {
		return err
	}

	getDAHTime, err := meter.Float64Histogram("eds_store_get_dah_time_histogram",
		metric.WithDescription("eds store get dah time histogram(s)"))
	if err != nil {
		return err
	}

	removeTime, err := meter.Float64Histogram("eds_store_remove_time_histogram",
		metric.WithDescription("eds store remove time histogram(s)"))
	if err != nil {
		return err
	}

	getTime, err := meter.Float64Histogram("eds_store_get_time_histogram",
		metric.WithDescription("eds store get time histogram(s)"))
	if err != nil {
		return err
	}

	hasTime, err := meter.Float64Histogram("eds_store_has_time_histogram",
		metric.WithDescription("eds store has time histogram(s)"))
	if err != nil {
		return err
	}

	listTime, err := meter.Float64Histogram("eds_store_list_time_histogram",
		metric.WithDescription("eds store list time histogram(s)"))
	if err != nil {
		return err
	}

	longOpTime, err := meter.Float64Histogram("eds_store_long_operation_time_histogram",
		metric.WithDescription("eds store long operation time histogram(s)"))
	if err != nil {
		return err
	}

	gcTime, err := meter.Float64Histogram("eds_store_gc_time",
		metric.WithDescription("dagstore gc time histogram(s)"))
	if err != nil {
		return err
	}

	if err = s.cache.EnableMetrics(); err != nil {
		return err
	}
	s.metrics = &metrics{
		putTime:              putTime,
		getCARTime:           getCARTime,
		getCARBlockstoreTime: getCARBlockstoreTime,
		getDAHTime:           getDAHTime,
		removeTime:           removeTime,
		getTime:              getTime,
		hasTime:              hasTime,
		listTime:             listTime,
		longOpTime:           longOpTime,
		gcTime:               gcTime,
	}
	return nil
}

func (m *metrics) observeGCtime(ctx context.Context, dur time.Duration, failed bool) {
	if m == nil {
		return
	}
	if ctx.Err() != nil {
		ctx = context.Background()
	}
	m.gcTime.Record(ctx, dur.Seconds(), metric.WithAttributes(
		attribute.Bool(failedKey, failed)))
}

func (m *metrics) observePut(ctx context.Context, dur time.Duration, result putResult, size uint) {
	if m == nil {
		return
	}
	if ctx.Err() != nil {
		ctx = context.Background()
	}

	m.putTime.Record(ctx, dur.Seconds(), metric.WithAttributes(
		attribute.String(putResultKey, string(result)),
		attribute.Int(sizeKey, int(size))))
}

func (m *metrics) observeLongOp(ctx context.Context, opName string, dur time.Duration, result longOpResult) {
	if m == nil {
		return
	}
	if ctx.Err() != nil {
		ctx = context.Background()
	}

	m.longOpTime.Record(ctx, dur.Seconds(), metric.WithAttributes(
		attribute.String(opNameKey, opName),
		attribute.String(longOpResultKey, string(result))))
}

func (m *metrics) observeGetCAR(ctx context.Context, dur time.Duration, failed bool) {
	if m == nil {
		return
	}
	if ctx.Err() != nil {
		ctx = context.Background()
	}

	m.getCARTime.Record(ctx, dur.Seconds(), metric.WithAttributes(
		attribute.Bool(failedKey, failed)))
}

func (m *metrics) observeCARBlockstore(ctx context.Context, dur time.Duration, failed bool) {
	if m == nil {
		return
	}
	if ctx.Err() != nil {
		ctx = context.Background()
	}

	m.getCARBlockstoreTime.Record(ctx, dur.Seconds(), metric.WithAttributes(
		attribute.Bool(failedKey, failed)))
}

func (m *metrics) observeGetDAH(ctx context.Context, dur time.Duration, failed bool) {
	if m == nil {
		return
	}
	if ctx.Err() != nil {
		ctx = context.Background()
	}

	m.getDAHTime.Record(ctx, dur.Seconds(), metric.WithAttributes(
		attribute.Bool(failedKey, failed)))
}

func (m *metrics) observeRemove(ctx context.Context, dur time.Duration, failed bool) {
	if m == nil {
		return
	}
	if ctx.Err() != nil {
		ctx = context.Background()
	}

	m.removeTime.Record(ctx, dur.Seconds(), metric.WithAttributes(
		attribute.Bool(failedKey, failed)))
}

func (m *metrics) observeGet(ctx context.Context, dur time.Duration, failed bool) {
	if m == nil {
		return
	}
	if ctx.Err() != nil {
		ctx = context.Background()
	}

	m.getTime.Record(ctx, dur.Seconds(), metric.WithAttributes(
		attribute.Bool(failedKey, failed)))
}

func (m *metrics) observeHas(ctx context.Context, dur time.Duration, failed bool) {
	if m == nil {
		return
	}
	if ctx.Err() != nil {
		ctx = context.Background()
	}

	m.hasTime.Record(ctx, dur.Seconds(), metric.WithAttributes(
		attribute.Bool(failedKey, failed)))
}

func (m *metrics) observeList(ctx context.Context, dur time.Duration, failed bool) {
	if m == nil {
		return
	}
	if ctx.Err() != nil {
		ctx = context.Background()
	}

	m.listTime.Record(ctx, dur.Seconds(), metric.WithAttributes(
		attribute.Bool(failedKey, failed)))
}
