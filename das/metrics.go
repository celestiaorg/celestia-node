package das

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric/global"
	"go.opentelemetry.io/otel/metric/instrument"
	"go.opentelemetry.io/otel/metric/instrument/asyncint64"
	"go.opentelemetry.io/otel/metric/instrument/syncfloat64"
	"go.opentelemetry.io/otel/metric/instrument/syncint64"

	"github.com/celestiaorg/celestia-node/header"
)

var (
	meter = global.MeterProvider().Meter("das")
)

type metrics struct {
	sampled    syncint64.Counter
	sampleTime syncfloat64.Histogram

	newHead syncint64.Counter

	busyWorkers        asyncint64.Gauge
	isRunning          asyncint64.Gauge
	headHeight         asyncint64.Gauge
	headOfSampledChain asyncint64.Gauge
}

func (sc *samplingCoordinator) initMetrics() error {
	sampled, err := meter.SyncInt64().Counter("das_sampled_headers_count",
		instrument.WithDescription("sampled headers counter"))
	if err != nil {
		return err
	}

	sampleTimeHist, err := meter.SyncFloat64().Histogram("das_sample_time_hist",
		instrument.WithDescription("header sampling time"))
	if err != nil {
		return err
	}

	newHead, err := meter.SyncInt64().Counter("das_head_updated",
		instrument.WithDescription("amount of times DAS'er advanced network head"))
	if err != nil {
		return err
	}

	busyWorkers, err := meter.AsyncInt64().Gauge("das_busy_workers_count",
		instrument.WithDescription("number of active parallel workers in DAS'er"))
	if err != nil {
		return err
	}

	isRunning, err := meter.AsyncInt64().Gauge("das_is_running",
		instrument.WithDescription("indicates whether das is running"))
	if err != nil {
		return err
	}

	headHeight, err := meter.AsyncInt64().Gauge("das_network_head_height",
		instrument.WithDescription("most recent network head height"))
	if err != nil {
		return err
	}

	headOfSampledChain, err := meter.AsyncInt64().Gauge("das_head_of_sampled_chain",
		instrument.WithDescription("all headers before das_head_of_sampled_chain were successfully sampled"))
	if err != nil {
		return err
	}

	err = meter.RegisterCallback(
		[]instrument.Asynchronous{
			busyWorkers, isRunning, headHeight, headOfSampledChain,
		},
		func(ctx context.Context) {
			stats, err := sc.stats(ctx)
			if err != nil {
				log.Errorf("observing stats: %s", err.Error())
			}

			busyWorkers.Observe(ctx, int64(len(stats.Workers)))
			headHeight.Observe(ctx, int64(stats.NetworkHead))
			headOfSampledChain.Observe(ctx, int64(stats.SampledChainHead))

			if stats.IsRunning {
				isRunning.Observe(ctx, 1)
			} else {
				isRunning.Observe(ctx, 0)
			}
		},
	)

	if err != nil {
		return fmt.Errorf("regestering metrics callback: %w", err)
	}

	sc.metrics = &metrics{
		sampled:            sampled,
		sampleTime:         sampleTimeHist,
		newHead:            newHead,
		busyWorkers:        busyWorkers,
		isRunning:          isRunning,
		headHeight:         headHeight,
		headOfSampledChain: headOfSampledChain,
	}
	return nil
}

func (m *metrics) observeSample(ctx context.Context, h *header.ExtendedHeader, sampleTime time.Duration, err error) {
	if m == nil {
		return
	}
	m.sampleTime.Record(ctx, sampleTime.Seconds(),
		attribute.Bool("failed", err != nil),
		attribute.Int("header_width", len(h.DAH.RowsRoots)))
	m.sampled.Add(ctx, 1,
		attribute.Bool("failed", err != nil),
		attribute.Int("header_width", len(h.DAH.RowsRoots)))
}

func (m *metrics) observeNewHead(ctx context.Context) {
	if m == nil {
		return
	}
	m.newHead.Add(ctx, 1)
}
