package system

import (
	"context"
	"fmt"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
)

var meter = otel.Meter("node/system")

type MetricCollector interface {
	Metrics() []metric.Observable
	Collect(context.Context, metric.Observer) error
}

type Metrics struct {
	collectors []MetricCollector
	reg        metric.Registration
}

func New() (*Metrics, error) {
	diskMetrics, err := newDiskMetrics()
	if err != nil {
		return nil, fmt.Errorf("disk metrics init: %w", err)
	}

	cpuMetrics, err := newCPUMetrics()
	if err != nil {
		return nil, fmt.Errorf("cpu metrics init: %w", err)
	}

	networkMetrics, err := newNetworkMetrics()
	if err != nil {
		return nil, fmt.Errorf("network metrics init: %w", err)
	}

	memoryMetrics, err := newMemoryMetrics()
	if err != nil {
		return nil, fmt.Errorf("memory metrics init: %w", err)
	}
	collectors := []MetricCollector{
		diskMetrics,
		cpuMetrics,
		networkMetrics,
		memoryMetrics,
	}

	var observables []metric.Observable
	for _, collector := range collectors {
		observables = append(observables, collector.Metrics()...)
	}

	reg, err := meter.RegisterCallback(
		func(ctx context.Context, observer metric.Observer) error {
			for _, collector := range collectors {
				if err := collector.Collect(ctx, observer); err != nil {
					return fmt.Errorf("%T collection failed: %w", collector, err)
				}
			}
			return nil
		},
		observables...,
	)
	if err != nil {
		return nil, fmt.Errorf("register callback: %w", err)
	}

	return &Metrics{
		collectors: collectors,
		reg:        reg,
	}, nil
}

func (m *Metrics) Stop() error {
	return m.reg.Unregister()
}
