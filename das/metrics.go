package das

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"github.com/celestiaorg/celestia-node/header"
)

const (
	jobTypeLabel     = "job_type"
	headerWidthLabel = "header_width"
	failedLabel      = "failed"
)

var (
	meter = otel.Meter("das")
)

type metrics struct {
	sampled       metric.Int64Counter
	sampleTime    metric.Float64Histogram
	getHeaderTime metric.Float64Histogram
	newHead       metric.Int64Counter

	lastSampledTS uint64
}

func (d *DASer) InitMetrics() error {
	sampled, err := meter.Int64Counter("das_sampled_headers_counter",
		metric.WithDescription("sampled headers counter"))
	if err != nil {
		return err
	}

	sampleTime, err := meter.Float64Histogram("das_sample_time_hist",
		metric.WithDescription("duration of sampling a single header"))
	if err != nil {
		return err
	}

	getHeaderTime, err := meter.Float64Histogram("das_get_header_time_hist",
		metric.WithDescription("duration of getting header from header store"))
	if err != nil {
		return err
	}

	newHead, err := meter.Int64Counter("das_head_updated_counter",
		metric.WithDescription("amount of times DAS'er advanced network head"))
	if err != nil {
		return err
	}

	lastSampledTS, err := meter.Int64ObservableGauge("das_latest_sampled_ts",
		metric.WithDescription("latest sampled timestamp"))
	if err != nil {
		return err
	}

	busyWorkers, err := meter.Int64ObservableGauge("das_busy_workers_amount",
		metric.WithDescription("number of active parallel workers in DAS'er"))
	if err != nil {
		return err
	}

	networkHead, err := meter.Int64ObservableGauge("das_network_head",
		metric.WithDescription("most recent network head"))
	if err != nil {
		return err
	}

	sampledChainHead, err := meter.Int64ObservableGauge("das_sampled_chain_head",
		metric.WithDescription("height of the sampled chain - all previous headers have been successfully sampled"))
	if err != nil {
		return err
	}

	totalSampled, err := meter.Int64ObservableGauge("das_total_sampled_headers",
		metric.WithDescription("total sampled headers gauge"),
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

	callback := func(ctx context.Context, observer metric.Observer) error {
		stats, err := d.sampler.stats(ctx)
		if err != nil {
			log.Errorf("observing stats: %s", err.Error())
			return err
		}

		for jobType, amount := range stats.workersByJobType() {
			observer.ObserveInt64(busyWorkers, amount,
				metric.WithAttributes(
					attribute.String(jobTypeLabel, string(jobType)),
				))
		}

		observer.ObserveInt64(networkHead, int64(stats.NetworkHead))
		observer.ObserveInt64(sampledChainHead, int64(stats.SampledChainHead))

		if ts := atomic.LoadUint64(&d.sampler.metrics.lastSampledTS); ts != 0 {
			observer.ObserveInt64(lastSampledTS, int64(ts))
		}

		observer.ObserveInt64(totalSampled, int64(stats.totalSampled()))
		return nil
	}

	_, err = meter.RegisterCallback(callback,
		lastSampledTS,
		busyWorkers,
		networkHead,
		sampledChainHead,
		totalSampled,
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
	if ctx.Err() != nil {
		ctx = context.Background()
	}
	m.sampleTime.Record(ctx, sampleTime.Seconds(),
		metric.WithAttributes(
			attribute.Bool(failedLabel, err != nil),
			attribute.Int(headerWidthLabel, len(h.DAH.RowRoots)),
			attribute.String(jobTypeLabel, string(jobType)),
		))

	m.sampled.Add(ctx, 1,
		metric.WithAttributes(
			attribute.Bool(failedLabel, err != nil),
			attribute.Int(headerWidthLabel, len(h.DAH.RowRoots)),
			attribute.String(jobTypeLabel, string(jobType)),
		))

	atomic.StoreUint64(&m.lastSampledTS, uint64(time.Now().UTC().Unix()))
}

// observeGetHeader records the time it took to get a header from the header store.
func (m *metrics) observeGetHeader(ctx context.Context, d time.Duration) {
	if m == nil {
		return
	}
	if ctx.Err() != nil {
		ctx = context.Background()
	}
	m.getHeaderTime.Record(ctx, d.Seconds())
}

// observeNewHead records the network head.
func (m *metrics) observeNewHead(ctx context.Context) {
	if m == nil {
		return
	}
	if ctx.Err() != nil {
		ctx = context.Background()
	}
	m.newHead.Add(ctx, 1)
}
