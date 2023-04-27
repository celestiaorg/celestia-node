package peers

import (
	"context"
	"fmt"
	"sync"
	"time"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/peer"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric/global"
	"go.opentelemetry.io/otel/metric/instrument"
	"go.opentelemetry.io/otel/metric/instrument/asyncint64"
	"go.opentelemetry.io/otel/metric/instrument/syncint64"

	"github.com/celestiaorg/celestia-node/share/p2p/shrexsub"
)

const (
	observeTimeout = 100 * time.Millisecond

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
	meter = global.MeterProvider().Meter("shrex_peer_manager")
)

type blacklistPeerReason string

type peerStatus string

type poolStatus string

type peerSource string

type metrics struct {
	getPeer                  syncint64.Counter   // attributes: source, is_instant
	getPeerWaitTimeHistogram syncint64.Histogram // attributes: source
	getPeerPoolSizeHistogram syncint64.Histogram // attributes: source
	doneResult               syncint64.Counter   // attributes: source, done_result
	validationResult         syncint64.Counter   // attributes: validation_result

	shrexPools               asyncint64.Gauge // attributes: pool_status
	fullNodesPool            asyncint64.Gauge // attributes: pool_status
	blacklistedPeersByReason sync.Map
	blacklistedPeers         asyncint64.Gauge // attributes: blacklist_reason
}

func initMetrics(manager *Manager) (*metrics, error) {
	getPeer, err := meter.SyncInt64().Counter("peer_manager_get_peer_counter",
		instrument.WithDescription("get peer counter"))
	if err != nil {
		return nil, err
	}

	getPeerWaitTimeHistogram, err := meter.SyncInt64().Histogram("peer_manager_get_peer_ms_time_hist",
		instrument.WithDescription("get peer time histogram(ms), observed only for async get(is_instant = false)"))
	if err != nil {
		return nil, err
	}

	getPeerPoolSizeHistogram, err := meter.SyncInt64().Histogram("peer_manager_get_peer_pool_size_hist",
		instrument.WithDescription("amount of available active peers in pool at time when get was called"))
	if err != nil {
		return nil, err
	}

	doneResult, err := meter.SyncInt64().Counter("peer_manager_done_result_counter",
		instrument.WithDescription("done results counter"))
	if err != nil {
		return nil, err
	}

	validationResult, err := meter.SyncInt64().Counter("peer_manager_validation_result_counter",
		instrument.WithDescription("validation result counter"))
	if err != nil {
		return nil, err
	}

	shrexPools, err := meter.AsyncInt64().Gauge("peer_manager_pools_gauge",
		instrument.WithDescription("pools amount"))
	if err != nil {
		return nil, err
	}

	fullNodesPool, err := meter.AsyncInt64().Gauge("peer_manager_full_nodes_gauge",
		instrument.WithDescription("full nodes pool peers amount"))
	if err != nil {
		return nil, err
	}

	blacklisted, err := meter.AsyncInt64().Gauge("peer_manager_blacklisted_peers",
		instrument.WithDescription("blacklisted peers amount"))
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

	err = meter.RegisterCallback(
		[]instrument.Asynchronous{
			shrexPools,
			fullNodesPool,
			blacklisted,
		},
		func(ctx context.Context) {
			for poolStatus, count := range manager.shrexPools() {
				shrexPools.Observe(ctx, count,
					attribute.String(poolStatusKey, string(poolStatus)))
			}

			fullNodesPool.Observe(ctx, int64(manager.fullNodes.len()),
				attribute.String(peerStatusKey, string(peerStatusActive)))
			fullNodesPool.Observe(ctx, int64(manager.fullNodes.cooldown.len()),
				attribute.String(peerStatusKey, string(peerStatusCooldown)))

			metrics.blacklistedPeersByReason.Range(func(key, value any) bool {
				reason := key.(blacklistPeerReason)
				amount := value.(int)
				blacklisted.Observe(ctx, int64(amount),
					attribute.String(blacklistPeerReasonKey, string(reason)))
				return true
			})
		},
	)

	if err != nil {
		return nil, fmt.Errorf("registering metrics callback: %w", err)
	}
	return metrics, nil
}

func (m *metrics) observeGetPeer(source peerSource, poolSize int, waitTime time.Duration) {
	if m == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), observeTimeout)
	defer cancel()
	m.getPeer.Add(ctx, 1,
		attribute.String(sourceKey, string(source)),
		attribute.Bool(isInstantKey, waitTime == 0))
	if source == sourceShrexSub {
		m.getPeerPoolSizeHistogram.Record(ctx, int64(poolSize),
			attribute.String(sourceKey, string(source)))
	}

	// record wait time only for async gets
	if waitTime > 0 {
		m.getPeerWaitTimeHistogram.Record(ctx, waitTime.Milliseconds(),
			attribute.String(sourceKey, string(source)))
	}
}

func (m *metrics) observeDoneResult(source peerSource, result result) {
	if m == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), observeTimeout)
	defer cancel()

	m.doneResult.Add(ctx, 1,
		attribute.String(sourceKey, string(source)),
		attribute.String(doneResultKey, string(result)))
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

		observeCtx, cancel := context.WithTimeout(context.Background(), observeTimeout)
		defer cancel()

		m.validationResult.Add(observeCtx, 1,
			attribute.String(validationResultKey, resStr))
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
