package system

import (
	"context"
	"fmt"
	"github.com/shirou/gopsutil/v3/mem"
	"go.opentelemetry.io/otel/metric"
)

type MemoryMetrics struct {
	// Virtual Memory metrics
	total     metric.Int64ObservableGauge
	used      metric.Int64ObservableGauge
	free      metric.Int64ObservableGauge
	shared    metric.Int64ObservableGauge
	cached    metric.Int64ObservableGauge
	available metric.Int64ObservableGauge

	// Swap metrics
	swapTotal metric.Int64ObservableGauge
	swapUsed  metric.Int64ObservableGauge
	swapFree  metric.Int64ObservableGauge
}

func newMemoryMetrics() (*MemoryMetrics, error) {
	total, err := meter.Int64ObservableGauge("system_memory_total_bytes",
		metric.WithDescription("Total physical memory"),
		metric.WithUnit("bytes"))
	if err != nil {
		return nil, fmt.Errorf("memory total metric: %w", err)
	}

	used, err := meter.Int64ObservableGauge("system_memory_used_bytes",
		metric.WithDescription("Used memory"),
		metric.WithUnit("bytes"))
	if err != nil {
		return nil, fmt.Errorf("memory used metric: %w", err)
	}

	free, err := meter.Int64ObservableGauge("system_memory_free_bytes",
		metric.WithDescription("Free memory"),
		metric.WithUnit("bytes"))
	if err != nil {
		return nil, fmt.Errorf("memory free metric: %w", err)
	}

	shared, err := meter.Int64ObservableGauge("system_memory_shared_bytes",
		metric.WithDescription("Shared memory"),
		metric.WithUnit("bytes"))
	if err != nil {
		return nil, fmt.Errorf("memory shared metric: %w", err)
	}

	cached, err := meter.Int64ObservableGauge("system_memory_cached_bytes",
		metric.WithDescription("Cached memory"),
		metric.WithUnit("bytes"))
	if err != nil {
		return nil, fmt.Errorf("memory cached metric: %w", err)
	}

	available, err := meter.Int64ObservableGauge("system_memory_available_bytes",
		metric.WithDescription("Available memory"),
		metric.WithUnit("bytes"))
	if err != nil {
		return nil, fmt.Errorf("memory available metric: %w", err)
	}

	swapTotal, err := meter.Int64ObservableGauge("system_swap_total_bytes",
		metric.WithDescription("Total swap space"),
		metric.WithUnit("bytes"))
	if err != nil {
		return nil, fmt.Errorf("swap total metric: %w", err)
	}

	swapUsed, err := meter.Int64ObservableGauge("system_swap_used_bytes",
		metric.WithDescription("Used swap space"),
		metric.WithUnit("bytes"))
	if err != nil {
		return nil, fmt.Errorf("swap used metric: %w", err)
	}

	swapFree, err := meter.Int64ObservableGauge("system_swap_free_bytes",
		metric.WithDescription("Free swap space"),
		metric.WithUnit("bytes"))
	if err != nil {
		return nil, fmt.Errorf("swap free metric: %w", err)
	}

	return &MemoryMetrics{
		total:     total,
		used:      used,
		free:      free,
		shared:    shared,
		cached:    cached,
		available: available,
		swapTotal: swapTotal,
		swapUsed:  swapUsed,
		swapFree:  swapFree,
	}, nil
}

func (m *MemoryMetrics) Metrics() []metric.Observable {
	return []metric.Observable{
		m.total,
		m.used,
		m.free,
		m.shared,
		m.cached,
		m.available,
		m.swapTotal,
		m.swapUsed,
		m.swapFree,
	}
}

func (m *MemoryMetrics) Collect(ctx context.Context, observer metric.Observer) error {
	// Get virtual memory stats
	vmem, err := mem.VirtualMemory()
	if err != nil {
		return fmt.Errorf("get virtual memory stats: %w", err)
	}

	observer.ObserveInt64(m.total, int64(vmem.Total))
	observer.ObserveInt64(m.used, int64(vmem.Used))
	observer.ObserveInt64(m.free, int64(vmem.Free))
	observer.ObserveInt64(m.shared, int64(vmem.Shared))
	observer.ObserveInt64(m.cached, int64(vmem.Cached))
	observer.ObserveInt64(m.available, int64(vmem.Available))

	// Get swap memory stats
	swap, err := mem.SwapMemory()
	if err != nil {
		return fmt.Errorf("get swap memory stats: %w", err)
	}

	observer.ObserveInt64(m.swapTotal, int64(swap.Total))
	observer.ObserveInt64(m.swapUsed, int64(swap.Used))
	observer.ObserveInt64(m.swapFree, int64(swap.Free))

	return nil
}
