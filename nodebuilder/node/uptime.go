// This file defines UptimeMetrics relative to the nodebuilder package.
package node

import (
	"context"
	"encoding/binary"
	"math"
	"time"

	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/namespace"
	"go.opentelemetry.io/otel/metric/global"
	"go.opentelemetry.io/otel/metric/instrument"
	"go.opentelemetry.io/otel/metric/instrument/asyncfloat64"
)

// UptimeMetrics is a struct that records
// 1. node start time: the timestamp when the node was started
// 2. node up time: total time the node has been running
//
// the node start time is recorded imperatively when RecordNodeStartTime is called
// whereas the node up time is recorded periodically upon callback recalling (re-mettering from optl)
type UptimeMetrics struct {
	// nodeStartTS is the timestamp when the node was started.
	nodeStartTS asyncfloat64.Gauge

	// totalNodeUptime is the total time the node has been running.
	totalNodeUptime asyncfloat64.Counter

	// store is the datastore used to store the node uptime metrics.
	store datastore.Datastore
}

var (
	meter              = global.MeterProvider().Meter("node")
	storePrefix        = datastore.NewKey("node")
	nodeStartTsKey     = datastore.NewKey("node_start_ts")
	totalNodeUpTimeKey = datastore.NewKey("total_node_uptime")
)

// NewUptimeMetrics creates a new UptimeMetrics
// and registers a callback to re-meter the totalNodeUptime metric.
func NewUptimeMetrics(ds datastore.Datastore) (*UptimeMetrics, error) {
	nodeStartTS, err := meter.
		AsyncFloat64().
		Gauge(
			"node_start_ts",
			instrument.WithDescription("timestamp when the node was started"),
		)
	if err != nil {
		return nil, err
	}

	totalNodeUptime, err := meter.
		AsyncFloat64().
		Counter(
			"node_uptime",
			instrument.WithDescription("total time the node has been running"),
		)
	if err != nil {
		return nil, err
	}

	m := &UptimeMetrics{
		nodeStartTS:     nodeStartTS,
		totalNodeUptime: totalNodeUptime,
		store:           namespace.Wrap(ds, storePrefix),
	}

	err = meter.RegisterCallback(
		[]instrument.Asynchronous{
			totalNodeUptime,
		},
		func(ctx context.Context) {
			totalNodeUptime.Observe(ctx, 1)
		},
	)
	if err != nil {
		return nil, err
	}

	return m, nil
}

// RecordNodeStartTime records the timestamp when the node was started.
func (m *UptimeMetrics) RecordNodeStartTime(ctx context.Context) {
	nodeStartTs, err := m.Get(ctx, nodeStartTsKey)

	if err == datastore.ErrNotFound {
		nodeStartTs = float64(time.Now().Unix())
		m.nodeStartTS.Observe(context.Background(), nodeStartTs)

		// persist to the datastore
		m.persist(ctx, nodeStartTsKey, nodeStartTs)

	}
}

// Persist persists the UptimeMetrics to the datastore
func (m *UptimeMetrics) persist(ctx context.Context, key datastore.Key, value float64) error {
	// represent the float64 number on an 8-bit big endian byte array
	bytes := make([]byte, 8)
	binary.BigEndian.PutUint64(bytes, math.Float64bits(value))

	// persist to the datastore
	if err := m.store.Put(ctx, key, bytes); err != nil {
		return err
	}

	return nil
}

// Get retrieves the value from the datastore
func (m *UptimeMetrics) Get(ctx context.Context, key datastore.Key) (float64, error) {
	bytes, err := m.store.Get(ctx, key)
	if err != nil {
		return 0, err
	}

	// convert the 8-bit big endian byte array to a float64 number
	bits := binary.BigEndian.Uint64(bytes)
	float := math.Float64frombits(bits)

	return float, nil
}
