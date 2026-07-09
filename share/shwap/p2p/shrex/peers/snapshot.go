package peers

import (
	"sort"
	"time"
)

// PeerSnapshot is a point-in-time view of a single peer's scoring and capacity state
// within a pool. It is a diagnostic projection of peerStats plus pool status.
type PeerSnapshot struct {
	ID     string `json:"id"`
	Status string `json:"status"`

	// InFlight is the number of requests currently assigned to this peer; Downloading
	// is true when InFlight > 0.
	InFlight    int  `json:"in_flight"`
	Downloading bool `json:"downloading"`

	// Quality is the load-independent quality score (higher is better); SelectionScore
	// is the value the selector actually maximizes (quality penalized by load/limits).
	Quality        float64 `json:"quality"`
	SelectionScore float64 `json:"selection_score"`

	// SuccessEWMA/LatencyEWMASeconds are the smoothed reliability and latency estimates.
	SuccessEWMA        float64 `json:"success_ewma"`
	LatencyEWMASeconds float64 `json:"latency_ewma_seconds"`

	// ConsecFails is the current consecutive-failure streak; TotalSuccess/TotalFailure
	// are cumulative outcome counts (how much this peer has served).
	ConsecFails  int `json:"consecutive_failures"`
	TotalSuccess int `json:"total_success"`
	TotalFailure int `json:"total_failure"`

	// ThroughputEWMABytesPerSec is the smoothed download throughput observed from this
	// peer (bytes/second); TotalBytes is the cumulative payload it has served.
	ThroughputEWMABytesPerSec float64 `json:"throughput_ewma_bytes_per_sec"`
	TotalBytes                int64   `json:"total_bytes"`

	// AtInflightCap is true when the peer is at or above the in-flight cap; RateLimited
	// is true when its client-side token bucket is exhausted.
	AtInflightCap bool    `json:"at_inflight_cap"`
	RateLimited   bool    `json:"rate_limited"`
	RateTokens    float64 `json:"rate_tokens"`

	// CooldownUntil is set when Status == "cooldown" and reports when the peer becomes
	// selectable again.
	CooldownUntil *time.Time `json:"cooldown_until,omitempty"`
}

// PoolSnapshot is a view of a single peer pool.
type PoolSnapshot struct {
	ActiveCount int            `json:"active_count"`
	TotalCount  int            `json:"total_count"`
	Peers       []PeerSnapshot `json:"peers"`
}

// DataHashPoolSnapshot summarizes a per-datahash shrexsub pool (peers that announced a
// specific block). Only counts are reported to keep the output bounded.
type DataHashPoolSnapshot struct {
	DataHash    string `json:"data_hash"`
	Height      uint64 `json:"height"`
	Validated   bool   `json:"validated"`
	ActiveCount int    `json:"active_count"`
	TotalCount  int    `json:"total_count"`
}

// ManagerSnapshot is a diagnostic snapshot of a peer Manager: its discovered-nodes pool
// (with full per-peer detail), a summary of its per-datahash shrexsub pools, and the
// number of blacklisted hashes.
type ManagerSnapshot struct {
	Tag               string                 `json:"tag"`
	Nodes             PoolSnapshot           `json:"nodes"`
	DataHashPools     []DataHashPoolSnapshot `json:"datahash_pools"`
	BlacklistedHashes int                    `json:"blacklisted_hashes"`
}

// Snapshot returns a diagnostic snapshot of the manager's state: the discovered-nodes
// pool with per-peer scores/stats/cooldowns, a per-datahash pool summary and the
// blacklist size. It is safe for concurrent use and does not mutate any state.
func (m *Manager) Snapshot() ManagerSnapshot {
	snap := ManagerSnapshot{
		Tag:   m.tag,
		Nodes: m.nodes.snapshot(),
	}

	m.lock.Lock()
	snap.BlacklistedHashes = len(m.blacklistedHashes)
	snap.DataHashPools = make([]DataHashPoolSnapshot, 0, len(m.pools))
	for hash, p := range m.pools {
		active, total := p.counts()
		snap.DataHashPools = append(snap.DataHashPools, DataHashPoolSnapshot{
			DataHash:    hash,
			Height:      p.height,
			Validated:   p.isValidatedDataHash.Load(),
			ActiveCount: active,
			TotalCount:  total,
		})
	}
	m.lock.Unlock()

	sort.Slice(snap.DataHashPools, func(i, j int) bool {
		return snap.DataHashPools[i].Height > snap.DataHashPools[j].Height
	})
	return snap
}

// counts returns the number of active and non-removed peers in the pool.
func (p *pool) counts() (activeCount, total int) {
	p.m.RLock()
	defer p.m.RUnlock()
	for _, peerID := range p.peersList {
		if p.statuses[peerID] != removed {
			total++
		}
	}
	return p.activeCount, total
}

// snapshot returns a full per-peer view of the pool. Peers are sorted by selection
// score, descending, so the peers most likely to be picked appear first.
func (p *pool) snapshot() PoolSnapshot {
	p.m.RLock()
	defer p.m.RUnlock()

	now := time.Now()
	snap := PoolSnapshot{
		ActiveCount: p.activeCount,
		Peers:       make([]PeerSnapshot, 0, len(p.peersList)),
	}

	for _, peerID := range p.peersList {
		st, ok := p.statuses[peerID]
		if !ok || st == removed {
			continue
		}
		snap.TotalCount++

		ps := PeerSnapshot{
			ID:     peerID.String(),
			Status: st.String(),
		}
		if stat := p.stats[peerID]; stat != nil {
			ps.InFlight = stat.inFlight
			ps.Downloading = stat.inFlight > 0
			ps.Quality = stat.quality(now)
			ps.SelectionScore = stat.selectionScore(now, p.params)
			ps.SuccessEWMA = stat.successEWMA
			ps.LatencyEWMASeconds = stat.latencyEWMA
			ps.ConsecFails = stat.consecFails
			ps.TotalSuccess = stat.totalSuccess
			ps.TotalFailure = stat.totalFailure
			ps.ThroughputEWMABytesPerSec = stat.throughputEWMA
			ps.TotalBytes = stat.totalBytes
			ps.AtInflightCap = stat.inFlight >= p.params.InflightCap
			ps.RateTokens = stat.limiter.Tokens()
			ps.RateLimited = ps.RateTokens < 1
		}
		if st == cooldown {
			if until, queued := p.cooldown.releaseAt(peerID); queued {
				ps.CooldownUntil = &until
			}
		}
		snap.Peers = append(snap.Peers, ps)
	}

	sort.Slice(snap.Peers, func(i, j int) bool {
		return snap.Peers[i].SelectionScore > snap.Peers[j].SelectionScore
	})
	return snap
}
