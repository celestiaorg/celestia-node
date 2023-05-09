package discovery

import (
	"context"
	"fmt"

	"github.com/libp2p/go-libp2p/core/peer"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric/global"
	"go.opentelemetry.io/otel/metric/instrument"
	"go.opentelemetry.io/otel/metric/instrument/asyncint64"
	"go.opentelemetry.io/otel/metric/instrument/syncint64"
)

const (
	discoveryEnougPeersKey  = "enough_peers"
	discoveryFindCancledKey = "is_canceled"

	handlePeerResultKey                    = "result"
	handlePeerSkipSelf    handlePeerResult = "skip_self"
	handlePeerEmptyAddrs  handlePeerResult = "skip_empty_addresses"
	handlePeerEnoughPeers handlePeerResult = "skip_enough_peers"
	handlePeerBackoff     handlePeerResult = "skip_backoff"
	handlePeerConnected   handlePeerResult = "connected"
	handlePeerConnErr     handlePeerResult = "conn_err"
	handlePeerInSet       handlePeerResult = "in_set"

	advertiseFailedKey = "failed"
)

var (
	meter = global.MeterProvider().Meter("share_discovery")
)

type handlePeerResult string

type metrics struct {
	peersAmount      asyncint64.Gauge
	discoveryResult  syncint64.Counter // attributes: enough_peers[bool],is_canceled[bool]
	discoveryStuck   syncint64.Counter
	handlePeerResult syncint64.Counter // attributes: result[string]
	advertise        syncint64.Counter // attributes: failed[bool]
	peerAdded        syncint64.Counter
	peerRemoved      syncint64.Counter
}

// WithMetrics turns on metric collection in discoery.
func (d *Discovery) WithMetrics() error {
	metrics, err := initMetrics(d)
	if err != nil {
		return fmt.Errorf("discovery: init metrics: %w", err)
	}
	d.metrics = metrics
	d.WithOnPeersUpdate(metrics.observeOnPeersUpdate)
	return nil
}

func initMetrics(d *Discovery) (*metrics, error) {
	peersAmount, err := meter.AsyncInt64().Gauge("discovery_amount_of_peers",
		instrument.WithDescription("amount of peers in discovery set"))
	if err != nil {
		return nil, err
	}

	discoveryResult, err := meter.SyncInt64().Counter("discovery_find_peers_result",
		instrument.WithDescription("result of find peers run"))
	if err != nil {
		return nil, err
	}

	discoveryStuck, err := meter.SyncInt64().Counter("discovery_lookup_is_stuck",
		instrument.WithDescription("indicates discovery wasn't able to find peers for more than 1 min"))
	if err != nil {
		return nil, err
	}

	handlePeerResultCounter, err := meter.SyncInt64().Counter("discovery_handler_peer_result",
		instrument.WithDescription("result handling found peer"))
	if err != nil {
		return nil, err
	}

	advertise, err := meter.SyncInt64().Counter("discovery_advertise_event",
		instrument.WithDescription("advertise events counter"))
	if err != nil {
		return nil, err
	}

	peerAdded, err := meter.SyncInt64().Counter("discovery_add_peer",
		instrument.WithDescription("add peer to discovery set counter"))
	if err != nil {
		return nil, err
	}

	peerRemoved, err := meter.SyncInt64().Counter("discovery_remove_peer",
		instrument.WithDescription("remove peer from discovery set counter"))
	if err != nil {
		return nil, err
	}

	metrics := &metrics{
		peersAmount:      peersAmount,
		discoveryResult:  discoveryResult,
		discoveryStuck:   discoveryStuck,
		handlePeerResult: handlePeerResultCounter,
		advertise:        advertise,
		peerAdded:        peerAdded,
		peerRemoved:      peerRemoved,
	}

	err = meter.RegisterCallback(
		[]instrument.Asynchronous{
			peersAmount,
		},
		func(ctx context.Context) {
			peersAmount.Observe(ctx, int64(d.set.Size()))
		},
	)
	if err != nil {
		return nil, fmt.Errorf("registering metrics callback: %w", err)
	}
	return metrics, nil
}

func (m *metrics) observeFindPeers(ctx context.Context, canceled, isEnoughPeers bool) {
	if m == nil {
		return
	}
	if ctx.Err() != nil {
		ctx = context.Background()
	}

	m.discoveryResult.Add(ctx, 1,
		attribute.Bool(discoveryFindCancledKey, canceled),
		attribute.Bool(discoveryEnougPeersKey, isEnoughPeers))
}

func (m *metrics) observeHandlePeer(ctx context.Context, result handlePeerResult) {
	if m == nil {
		return
	}
	if ctx.Err() != nil {
		ctx = context.Background()
	}

	m.handlePeerResult.Add(ctx, 1,
		attribute.String(handlePeerResultKey, string(result)))
}

func (m *metrics) observeAdvertise(ctx context.Context, err error) {
	if m == nil {
		return
	}
	if ctx.Err() != nil {
		ctx = context.Background()
	}

	m.advertise.Add(ctx, 1,
		attribute.Bool(advertiseFailedKey, err != nil))
}

func (m *metrics) observeOnPeersUpdate(_ peer.ID, isAdded bool) {
	if m == nil {
		return
	}
	ctx := context.Background()

	if isAdded {
		m.peerAdded.Add(ctx, 1)
		return
	}
	m.peerRemoved.Add(ctx, 1)
}

func (m *metrics) observeDiscoveryStuck(ctx context.Context) {
	if m == nil {
		return
	}
	if ctx.Err() != nil {
		ctx = context.Background()
	}

	m.discoveryStuck.Add(ctx, 1)
}
