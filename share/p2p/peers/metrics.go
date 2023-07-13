package peers

import (
	"context"
	"fmt"
	"sync"
	"time"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/peer"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"github.com/celestiaorg/celestia-node/share/p2p/shrexsub"
)

const (
	isInstantKey  = "is_instant"
	doneResultKey = "done_result"

	sourceKey                  = "source"
	sourceShrexSub  peerSource = "shrexsub"
	sourceFullNodes peerSource = "full_nodes"

	blacklistPeerReasonKey                     = "blacklist_reason"
	reasonInvalidHash      blacklistPeerReason = "invalid_hash"
	reasonMisbehave        blacklistPeerReason = "misbehave"

	validationResultKey = "validation_result"
	validationAccept    = "accept"
	validationReject    = "reject"
	validationIgnore    = "ignore"

	peerStatusKey                 = "peer_status"
	peerStatusActive   peerStatus = "active"
	peerStatusCooldown peerStatus = "cooldown"

	poolStatusKey                    = "pool_status"
	poolStatusCreated     poolStatus = "created"
	poolStatusValidated   poolStatus = "validated"
	poolStatusSynced      poolStatus = "synced"
	poolStatusBlacklisted poolStatus = "blacklisted"
	// Pool status model:
	//        	created(unvalidated)
	//  	/						\
	//  validated(unsynced)  	  blacklisted
	//			|
	//  	  synced
)

var (
	meter = otel.Meter("shrex_peer_manager")
)

type blacklistPeerReason string

type peerStatus string

type poolStatus string

type peerSource string

type metrics struct {
	getPeer                  metric.Int64Counter   // attributes: source, is_instant
	getPeerWaitTimeHistogram metric.Int64Histogram // attributes: source
	getPeerPoolSizeHistogram metric.Int64Histogram // attributes: source
	doneResult               metric.Int64Counter   // attributes: source, done_result
	validationResult         metric.Int64Counter   // attributes: validation_result

	shrexPools               metric.Int64ObservableGauge // attributes: pool_status
	fullNodesPool            metric.Int64ObservableGauge // attributes: pool_status
	blacklistedPeersByReason sync.Map
	blacklistedPeers         metric.Int64ObservableGauge // attributes: blacklist_reason
}

func initMetrics(manager *Manager) (*metrics, error) {
	getPeer, err := meter.Int64Counter("peer_manager_get_peer_counter",
		metric.WithDescription("get peer counter"))
	if err != nil {
		return nil, err
	}

	getPeerWaitTimeHistogram, err := meter.Int64Histogram("peer_manager_get_peer_ms_time_hist",
		metric.WithDescription("get peer time histogram(ms), observed only for async get(is_instant = false)"))
	if err != nil {
		return nil, err
	}

	getPeerPoolSizeHistogram, err := meter.Int64Histogram("peer_manager_get_peer_pool_size_hist",
		metric.WithDescription("amount of available active peers in pool at time when get was called"))
	if err != nil {
		return nil, err
	}

	doneResult, err := meter.Int64Counter("peer_manager_done_result_counter",
		metric.WithDescription("done results counter"))
	if err != nil {
		return nil, err
	}

	validationResult, err := meter.Int64Counter("peer_manager_validation_result_counter",
		metric.WithDescription("validation result counter"))
	if err != nil {
		return nil, err
	}

	shrexPools, err := meter.Int64ObservableGauge("peer_manager_pools_gauge",
		metric.WithDescription("pools amount"))
	if err != nil {
		return nil, err
	}

	fullNodesPool, err := meter.Int64ObservableGauge("peer_manager_full_nodes_gauge",
		metric.WithDescription("full nodes pool peers amount"))
	if err != nil {
		return nil, err
	}

	blacklisted, err := meter.Int64ObservableGauge("peer_manager_blacklisted_peers",
		metric.WithDescription("blacklisted peers amount"))
	if err != nil {
		return nil, err
	}

	metrics := &metrics{
		getPeer:                  getPeer,
		getPeerWaitTimeHistogram: getPeerWaitTimeHistogram,
		doneResult:               doneResult,
		validationResult:         validationResult,
		shrexPools:               shrexPools,
		fullNodesPool:            fullNodesPool,
		getPeerPoolSizeHistogram: getPeerPoolSizeHistogram,
		blacklistedPeers:         blacklisted,
	}

	callback := func(ctx context.Context, observer metric.Observer) error {
		for poolStatus, count := range manager.shrexPools() {
			observer.ObserveInt64(shrexPools, count,
				metric.WithAttributes(
					attribute.String(poolStatusKey, string(poolStatus))))
		}

		observer.ObserveInt64(fullNodesPool, int64(manager.fullNodes.len()),
			metric.WithAttributes(
				attribute.String(peerStatusKey, string(peerStatusActive))))
		observer.ObserveInt64(fullNodesPool, int64(manager.fullNodes.cooldown.len()),
			metric.WithAttributes(
				attribute.String(peerStatusKey, string(peerStatusCooldown))))

		metrics.blacklistedPeersByReason.Range(func(key, value any) bool {
			reason := key.(blacklistPeerReason)
			amount := value.(int)
			observer.ObserveInt64(blacklisted, int64(amount),
				metric.WithAttributes(
					attribute.String(blacklistPeerReasonKey, string(reason))))
			return true
		})
		return nil
	}
	_, err = meter.RegisterCallback(callback, shrexPools, fullNodesPool, blacklisted)
	if err != nil {
		return nil, fmt.Errorf("registering metrics callback: %w", err)
	}
	return metrics, nil
}

func (m *metrics) observeGetPeer(
	ctx context.Context,
	source peerSource, poolSize int, waitTime time.Duration,
) {
	if m == nil {
		return
	}
	if ctx.Err() != nil {
		ctx = context.Background()
	}
	m.getPeer.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String(sourceKey, string(source)),
			attribute.Bool(isInstantKey, waitTime == 0)))
	if source == sourceShrexSub {
		m.getPeerPoolSizeHistogram.Record(ctx, int64(poolSize),
			metric.WithAttributes(
				attribute.String(sourceKey, string(source))))
	}

	// record wait time only for async gets
	if waitTime > 0 {
		m.getPeerWaitTimeHistogram.Record(ctx, waitTime.Milliseconds(),
			metric.WithAttributes(
				attribute.String(sourceKey, string(source))))
	}
}

func (m *metrics) observeDoneResult(source peerSource, result result) {
	if m == nil {
		return
	}

	ctx := context.Background()
	m.doneResult.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String(sourceKey, string(source)),
			attribute.String(doneResultKey, string(result))))
}

// validationObserver is a middleware that observes validation results as metrics
func (m *metrics) validationObserver(validator shrexsub.ValidatorFn) shrexsub.ValidatorFn {
	if m == nil {
		return validator
	}
	return func(ctx context.Context, id peer.ID, n shrexsub.Notification) pubsub.ValidationResult {
		res := validator(ctx, id, n)

		var resStr string
		switch res {
		case pubsub.ValidationAccept:
			resStr = validationAccept
		case pubsub.ValidationReject:
			resStr = validationReject
		case pubsub.ValidationIgnore:
			resStr = validationIgnore
		default:
			resStr = "unknown"
		}

		if ctx.Err() != nil {
			ctx = context.Background()
		}

		m.validationResult.Add(ctx, 1,
			metric.WithAttributes(
				attribute.String(validationResultKey, resStr)))
		return res
	}
}

// observeBlacklistPeers stores amount of blacklisted peers by reason
func (m *metrics) observeBlacklistPeers(reason blacklistPeerReason, amount int) {
	if m == nil {
		return
	}
	for {
		prevVal, loaded := m.blacklistedPeersByReason.LoadOrStore(reason, amount)
		if !loaded {
			return
		}

		newVal := prevVal.(int) + amount
		if m.blacklistedPeersByReason.CompareAndSwap(reason, prevVal, newVal) {
			return
		}
	}
}

// shrexPools collects amount of shrex pools by poolStatus
func (m *Manager) shrexPools() map[poolStatus]int64 {
	m.lock.Lock()
	defer m.lock.Unlock()

	shrexPools := make(map[poolStatus]int64)
	for _, p := range m.pools {
		if !p.isValidatedDataHash.Load() {
			shrexPools[poolStatusCreated]++
			continue
		}

		if p.isSynced.Load() {
			shrexPools[poolStatusSynced]++
			continue
		}

		// pool is validated but not synced
		shrexPools[poolStatusValidated]++
	}

	shrexPools[poolStatusBlacklisted] = int64(len(m.blacklistedHashes))
	return shrexPools
}
