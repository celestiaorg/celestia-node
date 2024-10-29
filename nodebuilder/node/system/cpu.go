package system

import (
	"context"
	"fmt"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/load"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

type CPUMetrics struct {
	usage     metric.Float64ObservableGauge
	loadAvg1  metric.Float64ObservableGauge
	loadAvg5  metric.Float64ObservableGauge
	loadAvg15 metric.Float64ObservableGauge
	threads   metric.Int64ObservableGauge

	cpuInfo *cpu.InfoStat
}

func newCPUMetrics() (*CPUMetrics, error) {
	usage, err := meter.Float64ObservableGauge("system_cpu_usage",
		metric.WithDescription("CPU usage percentage"),
		metric.WithUnit("percent"))
	if err != nil {
		return nil, fmt.Errorf("cpu usage metric: %w", err)
	}

	loadAvg1, err := meter.Float64ObservableGauge("system_cpu_load1",
		metric.WithDescription("CPU load average 1 minute"),
		metric.WithUnit("load"))
	if err != nil {
		return nil, fmt.Errorf("cpu load1 metric: %w", err)
	}

	loadAvg5, err := meter.Float64ObservableGauge("system_cpu_load5",
		metric.WithDescription("CPU load average 5 minutes"),
		metric.WithUnit("load"))
	if err != nil {
		return nil, fmt.Errorf("cpu load5 metric: %w", err)
	}

	loadAvg15, err := meter.Float64ObservableGauge("system_cpu_load15",
		metric.WithDescription("CPU load average 15 minutes"),
		metric.WithUnit("load"))
	if err != nil {
		return nil, fmt.Errorf("cpu load15 metric: %w", err)
	}

	threads, err := meter.Int64ObservableGauge("system_cpu_threads",
		metric.WithDescription("Number of CPU threads"),
		metric.WithUnit("threads"))
	if err != nil {
		return nil, fmt.Errorf("cpu threads metric: %w", err)
	}

	// Get CPU info once during initialization
	cpuInfo, err := cpu.Info()
	if err != nil {
		return nil, fmt.Errorf("get cpu info: %w", err)
	}

	if len(cpuInfo) == 0 {
		return nil, fmt.Errorf("no cpu info available")
	}

	return &CPUMetrics{
		usage:     usage,
		loadAvg1:  loadAvg1,
		loadAvg5:  loadAvg5,
		loadAvg15: loadAvg15,
		threads:   threads,
		cpuInfo:   &cpuInfo[0],
	}, nil
}

func (c *CPUMetrics) Metrics() []metric.Observable {
	return []metric.Observable{
		c.usage,
		c.loadAvg1,
		c.loadAvg5,
		c.loadAvg15,
		c.threads,
	}
}

func (c *CPUMetrics) Collect(ctx context.Context, observer metric.Observer) error {
	percentages, err := cpu.Percent(0, false)
	if err != nil {
		return fmt.Errorf("get cpu percent: %w", err)
	}
	if len(percentages) > 0 {
		observer.ObserveFloat64(c.usage, percentages[0])
	}

	loadAvg, err := load.Avg()
	if err != nil {
		return fmt.Errorf("get load average: %w", err)
	}

	observer.ObserveFloat64(c.loadAvg1, loadAvg.Load1)
	observer.ObserveFloat64(c.loadAvg5, loadAvg.Load5)
	observer.ObserveFloat64(c.loadAvg15, loadAvg.Load15)

	threads, err := cpu.Counts(true)
	if err != nil {
		return fmt.Errorf("get cpu threads: %w", err)
	}

	observer.ObserveInt64(c.threads, int64(threads),
		metric.WithAttributes(
			attribute.String("model", c.cpuInfo.ModelName),
			attribute.String("vendor", c.cpuInfo.VendorID),
		))

	return nil
}
