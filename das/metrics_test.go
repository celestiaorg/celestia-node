package das

import (
	"context"
	"testing"
	"time"

	"github.com/ipfs/go-datastore"
	ds_sync "github.com/ipfs/go-datastore/sync"
	mdutils "github.com/ipfs/go-merkledag/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/exporters/stdout/stdoutmetric"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"

	"github.com/celestiaorg/celestia-node/share/availability/light"
	"github.com/celestiaorg/celestia-node/share/getters"
)

func TestMetrics_TotalSampled(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	t.Cleanup(cancel)

	daser, _, err := NewTestDASer(ctx, t)
	require.NoError(t, err)

	provider, reader, err := TestingMeterProvider()
	require.NoError(t, err)
	meter = provider.Meter("test")

	err = daser.InitMetrics()
	require.NoError(t, err)

	err = daser.Start(ctx)
	require.NoError(t, err)

	mr, err := reader.Collect(context.Background())
	require.NoError(t, err)

	assert.Equal(t, 1, len(mr.ScopeMetrics))
	assert.Equal(t, 4, len(mr.ScopeMetrics[0].Metrics))

	totalSampled := mr.ScopeMetrics[0].Metrics[3]
	totalSampledGauge, _ := (totalSampled.Data).(metricdata.Gauge[float64])

	// assert that the collected metrics were the correct onces
	assert.Equal(t, totalSampled.Name, "das_total_sampled_headers")

	// assert for data correctness
	assert.Equal(t, len(totalSampledGauge.DataPoints), 0)

	// wait for some DASing to happen
	// wait()

	// mr, err = reader.Collect(context.Background())
	// require.NoError(t, err)

	// totalSampled = mr.ScopeMetrics[0].Metrics[8]
	// totalSampledGauge, _ = (totalSampled.Data).(metricdata.Gauge[float64])

	// // assert that the collected metrics were the correct onces
	// assert.Equal(t, totalSampled.Name, "das_total_sampled_headers")

	// // assert for data correctness
	// assert.Equal(t, len(totalSampledGauge.DataPoints), 2)

	err = daser.Stop(ctx)
	require.NoError(t, err)
}

// test recordTotalSampled
func TestRecordTotalSampled(t *testing.T) {
	t.Skip()
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

func NewTestDASer(ctx context.Context, t *testing.T) (*DASer, func(), error) {
	ds := ds_sync.MutexWrap(datastore.NewMapDatastore())
	bServ := mdutils.Bserv()
	avail := light.TestAvailability(getters.NewIPLDGetter(bServ))
	// 15 headers from the past and 15 future headers
	mockGet, sub, mockService := createDASerSubcomponents(t, bServ, 15, 15)

	daser, err := NewDASer(avail, sub, mockGet, ds, mockService)

	wait := func() {
		select {
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		case <-mockGet.doneCh:
		}
	}
	// wait for mock to indicate that catchup is done

	return daser, wait, err
}
