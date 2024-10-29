package system

import (
	"context"
	"fmt"
	"github.com/shirou/gopsutil/v3/disk"
	"go.opentelemetry.io/otel/metric"
)

type DiskMetrics struct {
	readBytes   metric.Float64ObservableGauge
	writeBytes  metric.Float64ObservableGauge
	readIOPS    metric.Float64ObservableGauge
	writeIOPS   metric.Float64ObservableGauge
	ioWait      metric.Float64ObservableGauge
	pendingIOPS metric.Float64ObservableGauge
}

func newDiskMetrics() (*DiskMetrics, error) {
	readBytes, err := meter.Float64ObservableGauge("system_disk_read_bytes",
		metric.WithDescription("Disk bytes read"),
		metric.WithUnit("bytes"))
	if err != nil {
		return nil, fmt.Errorf("disk read metric: %w", err)
	}

	writeBytes, err := meter.Float64ObservableGauge("system_disk_write_bytes",
		metric.WithDescription("Disk bytes written"),
		metric.WithUnit("bytes/second"))
	if err != nil {
		return nil, fmt.Errorf("disk write metric: %w", err)
	}

	readIOPS, err := meter.Float64ObservableGauge("system_disk_read_iops",
		metric.WithDescription("Disk read IO operations"),
		metric.WithUnit("operation/second"))
	if err != nil {
		return nil, fmt.Errorf("disk read iops metric: %w", err)
	}

	writeIOPS, err := meter.Float64ObservableGauge("system_disk_write_iops",
		metric.WithDescription("Disk write IO operations"),
		metric.WithUnit("operation/second"))
	if err != nil {
		return nil, fmt.Errorf("disk write iops metric: %w", err)
	}

	ioWait, err := meter.Float64ObservableGauge("system_disk_io_wait",
		metric.WithDescription("Disk IO wait time"),
		metric.WithUnit("milliseconds"))
	if err != nil {
		return nil, fmt.Errorf("disk io wait metric: %w", err)
	}

	pendingIOPS, err := meter.Float64ObservableGauge("system_disk_pending_iops",
		metric.WithDescription("Disk pending IO operations"),
		metric.WithUnit("operation/second"))
	if err != nil {
		return nil, fmt.Errorf("disk pending iops metric: %w", err)
	}

	return &DiskMetrics{
		readBytes:   readBytes,
		writeBytes:  writeBytes,
		readIOPS:    readIOPS,
		writeIOPS:   writeIOPS,
		ioWait:      ioWait,
		pendingIOPS: pendingIOPS,
	}, nil
}

func (d *DiskMetrics) Metrics() []metric.Observable {
	return []metric.Observable{
		d.readBytes,
		d.writeBytes,
		d.readIOPS,
		d.writeIOPS,
		d.ioWait,
		d.pendingIOPS,
	}
}

func (d *DiskMetrics) Collect(ctx context.Context, observer metric.Observer) error {
	diskStats, err := disk.IOCounters()
	if err != nil {
		return fmt.Errorf("failed get disk stats: %w", err)
	}

	var totalReadBytes, totalWriteBytes float64
	var totalReadIOPS, totalWriteIOPS float64
	var totalIOWait, totalPendingIOPS float64

	for _, stat := range diskStats {
		totalReadBytes += float64(stat.ReadBytes)
		totalWriteBytes += float64(stat.WriteBytes)
		totalReadIOPS += float64(stat.ReadCount)
		totalWriteIOPS += float64(stat.WriteCount)
		totalIOWait += float64(stat.IoTime)
		totalPendingIOPS += float64(stat.IopsInProgress)
	}

	observer.ObserveFloat64(d.readBytes, totalReadBytes)
	observer.ObserveFloat64(d.writeBytes, totalWriteBytes)
	observer.ObserveFloat64(d.readIOPS, totalReadIOPS)
	observer.ObserveFloat64(d.writeIOPS, totalWriteIOPS)
	observer.ObserveFloat64(d.ioWait, totalIOWait)
	observer.ObserveFloat64(d.pendingIOPS, totalPendingIOPS)

	return nil
}
