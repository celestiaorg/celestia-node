package das

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric/global"
	"go.opentelemetry.io/otel/metric/instrument"
	"go.opentelemetry.io/otel/metric/instrument/syncfloat64"
	"go.opentelemetry.io/otel/metric/instrument/syncint64"

	"github.com/celestiaorg/celestia-node/header"
)

const (
	jobTypeLabel     = "job_type"
	headerWidthLabel = "header_width"
	failedLabel      = "failed"
)

var (
	meter = global.MeterProvider().Meter("das")
)

type metrics struct {
	sampled       syncint64.Counter
	sampleTime    syncfloat64.Histogram
	getHeaderTime syncfloat64.Histogram
	newHead       syncint64.Counter

	lastSampledTS uint64
}

func (d *DASer) InitMetrics() error {
	sampled, err := meter.SyncInt64().Counter("das_sampled_headers_counter",
		instrument.WithDescription("sampled headers counter"))
	if err != nil {
		return err
	}

	sampleTime, err := meter.SyncFloat64().Histogram("das_sample_time_hist",
		instrument.WithDescription("duration of sampling a single header"))
	if err != nil {
		return err
	}

	getHeaderTime, err := meter.SyncFloat64().Histogram("das_get_header_time_hist",
		instrument.WithDescription("duration of getting header from header store"))
	if err != nil {
		return err
	}

	newHead, err := meter.SyncInt64().Counter("das_head_updated_counter",
		instrument.WithDescription("amount of times DAS'er advanced network head"))
	if err != nil {
		return err
	}

	lastSampledTS, err := meter.AsyncInt64().Gauge("das_latest_sampled_ts",
		instrument.WithDescription("latest sampled timestamp"))
	if err != nil {
		return err
	}

	busyWorkers, err := meter.AsyncInt64().Gauge("das_busy_workers_amount",
		instrument.WithDescription("number of active parallel workers in DAS'er"))
	if err != nil {
		return err
	}

	networkHead, err := meter.AsyncInt64().Gauge("das_network_head",
		instrument.WithDescription("most recent network head"))
	if err != nil {
		return err
	}

	sampledChainHead, err := meter.AsyncInt64().Gauge("das_sampled_chain_head",
		instrument.WithDescription("height of the sampled chain - all previous headers have been successfully sampled"))
	if err != nil {
		return err
	}

	totalSampled, err := meter.
		AsyncInt64().
		Gauge(
			"das_total_sampled_headers",
			instrument.WithDescription("total sampled headers gauge"),
		)
	if err != nil {
		return err
	}

	d.sampler.metrics = &metrics{
		sampled:       sampled,
		sampleTime:    sampleTime,
		getHeaderTime: getHeaderTime,
		newHead:       newHead,
	}

	err = meter.RegisterCallback(
		[]instrument.Asynchronous{
			lastSampledTS,
			busyWorkers,
			networkHead,
			sampledChainHead,
			totalSampled,
		},
		func(ctx context.Context) {
			stats, err := d.sampler.stats(ctx)
			if err != nil {
				log.Errorf("observing stats: %s", err.Error())
			}

			for jobType, amount := range stats.workersByJobType() {
				busyWorkers.Observe(ctx, amount,
					attribute.String(jobTypeLabel, string(jobType)))
			}

			networkHead.Observe(ctx, int64(stats.NetworkHead))
			sampledChainHead.Observe(ctx, int64(stats.SampledChainHead))

			if ts := atomic.LoadUint64(&d.sampler.metrics.lastSampledTS); ts != 0 {
				lastSampledTS.Observe(ctx, int64(ts))
			}

			totalSampled.Observe(ctx, int64(stats.totalSampled()))
		},
	)

	if err != nil {
		return fmt.Errorf("registering metrics callback: %w", err)
	}

	return nil
}

// observeSample records the time it took to sample a header +
// the amount of sampled contiguous headers
func (m *metrics) observeSample(
	ctx context.Context,
	h *header.ExtendedHeader,
	sampleTime time.Duration,
	jobType jobType,
	err error,
) {
	if m == nil {
		return
	}
	m.sampleTime.Record(ctx, sampleTime.Seconds(),
		attribute.Bool(failedLabel, err != nil),
		attribute.Int(headerWidthLabel, len(h.DAH.RowsRoots)),
		attribute.String(jobTypeLabel, string(jobType)),
	)

	m.sampled.Add(ctx, 1,
		attribute.Bool(failedLabel, err != nil),
		attribute.Int(headerWidthLabel, len(h.DAH.RowsRoots)),
		attribute.String(jobTypeLabel, string(jobType)),
	)

	atomic.StoreUint64(&m.lastSampledTS, uint64(time.Now().UTC().Unix()))
}

// observeGetHeader records the time it took to get a header from the header store.
func (m *metrics) observeGetHeader(ctx context.Context, d time.Duration) {
	if m == nil {
		return
	}
	m.getHeaderTime.Record(ctx, d.Seconds())
}

// observeNewHead records the network head.
func (m *metrics) observeNewHead(ctx context.Context) {
	if m == nil {
		return
	}
	m.newHead.Add(ctx, 1)
}
