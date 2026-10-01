package das

import (
	"context"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/ipfs/go-datastore"
	ds_sync "github.com/ipfs/go-datastore/sync"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"

	"github.com/celestiaorg/celestia-node/share/availability/mocks"
)

func TestHeadLagOf(t *testing.T) {
	tests := []struct {
		name  string
		stats SamplingStats
		want  int64
	}{
		{"behind", SamplingStats{SampledChainHead: 10, NetworkHead: 100}, 90},
		{"caught up", SamplingStats{SampledChainHead: 100, NetworkHead: 100}, 0},
		{"ahead of network head", SamplingStats{SampledChainHead: 101, NetworkHead: 100}, 0},
		{"nothing sampled", SamplingStats{SampledChainHead: 0, NetworkHead: 0}, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, headLagOf(tt.stats))
		})
	}
}

// TestHeadLagMetricIsCollected ensures the gauge is registered in the metrics
// callback and actually reaches metric collection.
func TestHeadLagMetricIsCollected(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	otel.SetMeterProvider(sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader)))

	ds := ds_sync.MutexWrap(datastore.NewMapDatastore())
	ctrl := gomock.NewController(t)
	avail := mocks.NewMockAvailability(ctrl)
	avail.EXPECT().SharesAvailable(gomock.Any(), gomock.Any()).AnyTimes().Return(nil)
	mockGet, sub := createDASerSubcomponents(t, 15, 15)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	t.Cleanup(cancel)

	daser, err := NewDASer(avail, sub, mockGet, ds)
	require.NoError(t, err)
	require.NoError(t, daser.InitMetrics())

	require.NoError(t, daser.Start(ctx))
	t.Cleanup(func() {
		require.NoError(t, daser.Stop(context.Background()))
	})

	require.Eventually(t, func() bool {
		var rm metricdata.ResourceMetrics
		if err := reader.Collect(ctx, &rm); err != nil {
			return false
		}
		for _, sm := range rm.ScopeMetrics {
			for _, m := range sm.Metrics {
				if m.Name == "das_sampling_head_lag" {
					return true
				}
			}
		}
		return false
	}, timeout, 50*time.Millisecond, "das_sampling_head_lag was not collected")
}
