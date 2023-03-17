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

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/p2p/shrexsub"
)

const (
	observeTimeout = 100 * time.Millisecond

	isInstantKey  = "is_instant"
	doneResultKey = "done_result"

	sourceKey                  = "source"
	sourceShrexSub  peerSource = "shrexsub"
	sourceDiscovery peerSource = "discovery"

	blacklistPeerReasonKey                     = "blacklist_reason"
	reasonInvalidHash      blacklistPeerReason = "invalid_hash"
	reasonMisbehave        blacklistPeerReason = "misbehave"

	validationResultKey = "validation_result"
	validationAccept    = "accept"
	validationReject    = "reject"
	validationIgnore    = "ignore"

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

type poolStatus string

type peerSource string

type metrics struct {
	getPeer                  syncint64.Counter   // attributes: source, is_instant
	getPeerWaitTimeHistogram syncint64.Histogram // attributes: source
	doneResult               syncint64.Counter   // attributes: source, done_result
	validationResult         syncint64.Counter   // attributes: validation_result

	pools                    asyncint64.Gauge // attributes: pool_status
	peers                    asyncint64.Gauge // attributes: pool_status
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

	pools, err := meter.AsyncInt64().Gauge("peer_manager_pools_gauge",
		instrument.WithDescription("pools amount"))
	if err != nil {
		return nil, err
	}

	peers, err := meter.AsyncInt64().Gauge("peer_manager_peers_gauge",
		instrument.WithDescription("peers amount"))
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
		pools:                    pools,
		peers:                    peers,
		blacklistedPeers:         blacklisted,
	}

	err = meter.RegisterCallback(
		[]instrument.Asynchronous{
			pools,
			peers,
			blacklisted,
		},
		func(ctx context.Context) {
			stats := manager.stats()

			for poolStatus, count := range stats.pools {
				pools.Observe(ctx, count,
					attribute.String(poolStatusKey, string(poolStatus)))
			}

			for poolStatus, count := range stats.peers {
				peers.Observe(ctx, count,
					attribute.String(poolStatusKey, string(poolStatus)))
			}

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
		return nil, fmt.Errorf("regestering metrics callback: %w", err)
	}
	return metrics, nil
}

func (m *metrics) observeGetPeer(source peerSource, waitTime int64) {
	if m == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), observeTimeout)
	defer cancel()

	if waitTime > 0 {
		m.getPeerWaitTimeHistogram.Record(ctx, waitTime,
			attribute.String(sourceKey, string(source)))
	}

	m.getPeer.Add(ctx, 1,
		attribute.String(sourceKey, string(source)),
		attribute.Bool(isInstantKey, waitTime == 0))
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

func (m *metrics) validationObserver(validator shrexsub.Validator) shrexsub.Validator {
	if m == nil {
		return validator
	}
	return func(ctx context.Context, id peer.ID, datahash share.DataHash) pubsub.ValidationResult {
		res := validator(ctx, id, datahash)

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

type stats struct {
	pools map[poolStatus]int64
	peers map[poolStatus]int64
}

func (s stats) add(p *syncPool, status poolStatus) {
	s.pools[status]++
	s.peers[status] += int64(p.activeCount + p.cooldown.len())
}

func (m *Manager) stats() stats {
	m.lock.Lock()
	defer m.lock.Unlock()

	stats := stats{
		pools: make(map[poolStatus]int64),
		peers: make(map[poolStatus]int64),
	}

	for _, p := range m.pools {
		if !p.isValidatedDataHash.Load() {
			stats.add(p, poolStatusCreated)
			continue
		}

		if p.isSynced.Load() {
			stats.add(p, poolStatusSynced)
			continue
		}

		// pool is validated but not synced
		stats.add(p, poolStatusValidated)
	}

	stats.pools[poolStatusBlacklisted] = int64(len(m.blacklistedHashes))
	return stats
}
