package discovery

import (
	"context"
	"fmt"

	"github.com/libp2p/go-libp2p/core/peer"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

const (
	discoveryEnoughPeersKey = "enough_peers"

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
	meter = otel.Meter("share_discovery")
)

type handlePeerResult string

type metrics struct {
	peersAmount      metric.Int64ObservableGauge
	discoveryResult  metric.Int64Counter // attributes: enough_peers[bool],is_canceled[bool]
	handlePeerResult metric.Int64Counter // attributes: result[string]
	advertise        metric.Int64Counter // attributes: failed[bool]
	peerAdded        metric.Int64Counter
	peerRemoved      metric.Int64Counter
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
	peersAmount, err := meter.Int64ObservableGauge("discovery_amount_of_peers",
		metric.WithDescription("amount of peers in discovery set"))
	if err != nil {
		return nil, err
	}

	discoveryResult, err := meter.Int64Counter("discovery_find_peers_result",
		metric.WithDescription("result of find peers run"))
	if err != nil {
		return nil, err
	}

	handlePeerResultCounter, err := meter.Int64Counter("discovery_handler_peer_result",
		metric.WithDescription("result handling found peer"))
	if err != nil {
		return nil, err
	}

	advertise, err := meter.Int64Counter("discovery_advertise_event",
		metric.WithDescription("advertise events counter"))
	if err != nil {
		return nil, err
	}

	peerAdded, err := meter.Int64Counter("discovery_add_peer",
		metric.WithDescription("add peer to discovery set counter"))
	if err != nil {
		return nil, err
	}

	peerRemoved, err := meter.Int64Counter("discovery_remove_peer",
		metric.WithDescription("remove peer from discovery set counter"))
	if err != nil {
		return nil, err
	}

	backOffSize, err := meter.Int64ObservableGauge("discovery_backoff_amount",
		metric.WithDescription("amount of peers in backoff"))
	if err != nil {
		return nil, err
	}

	metrics := &metrics{
		peersAmount:      peersAmount,
		discoveryResult:  discoveryResult,
		handlePeerResult: handlePeerResultCounter,
		advertise:        advertise,
		peerAdded:        peerAdded,
		peerRemoved:      peerRemoved,
	}

	callback := func(ctx context.Context, observer metric.Observer) error {
		observer.ObserveInt64(peersAmount, int64(d.set.Size()))
		observer.ObserveInt64(backOffSize, int64(d.connector.Size()))
		return nil
	}
	_, err = meter.RegisterCallback(callback, peersAmount, backOffSize)
	if err != nil {
		return nil, fmt.Errorf("registering metrics callback: %w", err)
	}
	return metrics, nil
}

func (m *metrics) observeFindPeers(ctx context.Context, isEnoughPeers bool) {
	if m == nil {
		return
	}
	if ctx.Err() != nil {
		ctx = context.Background()
	}

	m.discoveryResult.Add(ctx, 1,
		metric.WithAttributes(
			attribute.Bool(discoveryEnoughPeersKey, isEnoughPeers)))
}

func (m *metrics) observeHandlePeer(ctx context.Context, result handlePeerResult) {
	if m == nil {
		return
	}
	if ctx.Err() != nil {
		ctx = context.Background()
	}

	m.handlePeerResult.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String(handlePeerResultKey, string(result))))
}

func (m *metrics) observeAdvertise(ctx context.Context, err error) {
	if m == nil {
		return
	}
	if ctx.Err() != nil {
		ctx = context.Background()
	}

	m.advertise.Add(ctx, 1,
		metric.WithAttributes(
			attribute.Bool(advertiseFailedKey, err != nil)))
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
