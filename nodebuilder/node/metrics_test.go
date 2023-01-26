package node

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/exporters/stdout/stdoutmetric"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

func TestWithMetrics(t *testing.T) {
	provider, reader, err := TestingMeterProvider()
	require.NoError(t, err)

	// re-assign the global variable `meter` from metrics.go
	meter = provider.Meter("test")
	err = WithMetrics()
	require.NoError(t, err)

	mr, err := reader.Collect(context.Background())
	require.NoError(t, err)

	// assert that the metrics are being collected
	assert.Equal(t, 1, len(mr.ScopeMetrics))
	assert.Equal(t, 2, len(mr.ScopeMetrics[0].Metrics))

	// access the metrics that have been collected since the last `Collect`
	nodeStartTS := mr.ScopeMetrics[0].Metrics[0]
	totalNodeRunTime := mr.ScopeMetrics[0].Metrics[1]

	// assert that the collected metrics were the right ones
	assert.Equal(t, nodeStartTS.Name, "node_start_ts")
	assert.Equal(t, totalNodeRunTime.Name, "node_runtime_counter_in_seconds")

	// retrieve metric values by casting them to their proper types
	nodeStartTSCounter, _ := (nodeStartTS.Data).(metricdata.Gauge[float64])
	totalNodeRunTimeGauge, _ := (totalNodeRunTime.Data).(metricdata.Sum[float64])

	// assert for correctness collected data
	start := float64(time.Now().Unix())
	assert.Equal(t, totalNodeRunTimeGauge.DataPoints[0].Value, 0.0)
	assert.Equal(t, nodeStartTSCounter.DataPoints[0].Value, start)

	// Recollect after 2 seconds
	<-time.After(2 * time.Second)
	mr, err = reader.Collect(context.Background())
	require.NoError(t, err)

	// assert that have been collected in the second collect
	// were the right ones (in number and attributes)
	assert.Equal(t, 1, len(mr.ScopeMetrics))
	// the fact that there is only one collected metrics is enough to verify
	// that node_start_ts was not collected again
	assert.Equal(t, 1, len(mr.ScopeMetrics[0].Metrics))

	totalNodeRunTime = mr.ScopeMetrics[0].Metrics[0]
	totalNodeRunTimeGauge, _ = (totalNodeRunTime.Data).(metricdata.Sum[float64])

	// assert that the collected metrics were the right ones
	assert.Equal(t, totalNodeRunTime.Name, "node_runtime_counter_in_seconds")
	//  assert for correctness collected data
	assert.Equal(t, totalNodeRunTimeGauge.DataPoints[0].Value, float64(time.Now().Unix()-int64(start)))
}

func TestingMeterProvider() (*metric.MeterProvider, metric.Reader, error) {
	exp, err := stdoutmetric.New()
	if err != nil {
		return nil, nil, err
	}

	reader := metric.NewPeriodicReader(exp, metric.WithTimeout(1*time.Second))

	provider := metric.NewMeterProvider(
		metric.WithReader(reader),
	)

	return provider, reader, nil
}
