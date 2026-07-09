package peers

import (
	"context"
	"math/rand/v2"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
)

const defaultCleanupThreshold = 2

// pool stores peers and hands them out weighted by observed quality (ADR-014). It
// tracks per-peer stats (success/latency EWMA, in-flight load, adaptive cooldown) and
// selects via Power-of-Two-Choices so requests concentrate on good peers while still
// spreading load enough to respect per-peer rate/stream limits.
type pool struct {
	m           sync.RWMutex
	params      *Parameters
	peersList   []peer.ID
	statuses    map[peer.ID]status
	stats       map[peer.ID]*peerStats
	cooldown    *timedQueue
	activeCount int

	hasPeer   bool
	hasPeerCh chan struct{}

	cleanupThreshold int
}

type status int

const (
	active status = iota
	cooldown
	removed
)

func (s status) String() string {
	switch s {
	case active:
		return "active"
	case cooldown:
		return "cooldown"
	case removed:
		return "removed"
	default:
		return "unknown"
	}
}

// newPool returns new empty pool.
func newPool(params *Parameters) *pool {
	p := &pool{
		params:           params,
		peersList:        make([]peer.ID, 0),
		statuses:         make(map[peer.ID]status),
		stats:            make(map[peer.ID]*peerStats),
		hasPeerCh:        make(chan struct{}),
		cleanupThreshold: defaultCleanupThreshold,
	}
	p.cooldown = newTimedQueue(params.PeerCooldown, p.afterCooldown)
	return p
}

// tryGet returns a peer selected by Power-of-Two-Choices over the currently active
// peers, weighted by quality and current load. The returned bool indicates success.
// It does not mutate load accounting; the caller must call acquire once it commits to
// using the returned peer, and release when done.
func (p *pool) tryGet() (peer.ID, bool) {
	p.m.Lock()
	defer p.m.Unlock()

	if p.activeCount == 0 {
		return "", false
	}

	candidates := make([]peer.ID, 0, p.activeCount)
	for _, peerID := range p.peersList {
		if p.statuses[peerID] == active {
			candidates = append(candidates, peerID)
		}
	}
	if len(candidates) == 0 {
		return "", false
	}

	return p.pickP2C(candidates), true
}

// pickP2C draws up to P2CSampleSize random candidates and returns the highest scoring
// one. Must be called with the lock held and candidates non-empty.
func (p *pool) pickP2C(candidates []peer.ID) peer.ID {
	now := time.Now()
	k := p.params.P2CSampleSize
	if k > len(candidates) {
		k = len(candidates)
	}

	var (
		best      peer.ID
		bestScore = -1.0
		found     bool
	)
	eval := func(peerID peer.ID) {
		st := p.stats[peerID]
		if st == nil {
			return
		}
		score := st.selectionScore(now, p.params)
		if !found || score > bestScore {
			best, bestScore, found = peerID, score, true
		}
	}

	if len(candidates) <= k {
		for _, peerID := range candidates {
			eval(peerID)
		}
	} else {
		seen := make(map[int]struct{}, k)
		for len(seen) < k {
			idx := rand.IntN(len(candidates))
			if _, ok := seen[idx]; ok {
				continue
			}
			seen[idx] = struct{}{}
			eval(candidates[idx])
		}
	}

	if !found {
		// stats missing for all sampled (shouldn't happen); fall back to first candidate
		return candidates[0]
	}
	return best
}

// acquire records that the caller has committed to using peerID for a request: it
// increments the peer's in-flight count and consumes a rate-limiter token (best effort).
func (p *pool) acquire(peerID peer.ID) {
	p.m.Lock()
	defer p.m.Unlock()

	st := p.stats[peerID]
	if st == nil {
		return
	}
	st.inFlight++
	st.lastUpdate = time.Now()
	// best-effort token consumption; never blocks selection
	st.limiter.Allow()
}

// decInFlight decrements the peer's outstanding-request counter. It is kept separate
// from recordOutcome because the getter may report a result more than once for a single
// handout (a successful fetch that then fails verification); the caller guards this so
// in-flight is decremented exactly once while the outcome may be recorded twice.
func (p *pool) decInFlight(peerID peer.ID) {
	p.m.Lock()
	defer p.m.Unlock()

	if st := p.stats[peerID]; st != nil && st.inFlight > 0 {
		st.inFlight--
	}
}

// recordOutcome folds a request outcome into the peer's EWMAs and, on failure, puts the
// peer on an adaptive (escalating) cooldown if it is currently active.
func (p *pool) recordOutcome(peerID peer.ID, success bool, latency time.Duration, bytes int64) {
	p.m.Lock()
	defer p.m.Unlock()

	now := time.Now()
	st := p.stats[peerID]
	if st != nil {
		if success {
			st.recordSuccess(latency, bytes, p.params, now)
		} else {
			st.recordFailure(p.params, now)
		}
	}

	if success {
		return
	}

	// on failure, put the peer on an adaptive cooldown if it is currently active
	if s, ok := p.statuses[peerID]; ok && s == active {
		ttl := p.params.PeerCooldown
		if st != nil {
			ttl = st.adaptiveCooldown(p.params)
		}
		p.cooldown.push(peerID, ttl)
		p.statuses[peerID] = cooldown
		p.activeCount--
		p.checkHasPeers()
	}
}

// next sends a peer to the returned channel when it becomes available.
func (p *pool) next(ctx context.Context) <-chan peer.ID {
	peerCh := make(chan peer.ID, 1)
	go func() {
		for {
			if peerID, ok := p.tryGet(); ok {
				peerCh <- peerID
				return
			}

			p.m.RLock()
			hasPeerCh := p.hasPeerCh
			p.m.RUnlock()
			select {
			case <-hasPeerCh:
			case <-ctx.Done():
				return
			}
		}
	}()
	return peerCh
}

func (p *pool) add(peers ...peer.ID) {
	p.m.Lock()
	defer p.m.Unlock()

	now := time.Now()
	for _, peerID := range peers {
		status, ok := p.statuses[peerID]
		if ok && status != removed {
			continue
		}

		if !ok {
			p.peersList = append(p.peersList, peerID)
		}

		p.statuses[peerID] = active
		p.activeCount++
		if _, has := p.stats[peerID]; !has {
			// keep prior stats on re-add; only create for genuinely new peers
			p.stats[peerID] = newPeerStats(p.params, now)
		}
	}
	p.checkHasPeers()
}

func (p *pool) remove(peers ...peer.ID) {
	p.m.Lock()
	defer p.m.Unlock()

	for _, peerID := range peers {
		if status, ok := p.statuses[peerID]; ok && status != removed {
			p.statuses[peerID] = removed
			if status == active {
				p.activeCount--
			}
		}
	}

	// do cleanup if too much garbage
	if len(p.peersList) >= p.activeCount+p.cleanupThreshold {
		p.cleanup()
	}
	p.checkHasPeers()
}

func (p *pool) has(peer peer.ID) bool {
	p.m.RLock()
	defer p.m.RUnlock()

	status, ok := p.statuses[peer]
	return ok && status != removed
}

func (p *pool) peers() []peer.ID {
	p.m.RLock()
	defer p.m.RUnlock()

	peers := make([]peer.ID, 0, len(p.peersList))
	for peer, status := range p.statuses {
		if status != removed {
			peers = append(peers, peer)
		}
	}
	return peers
}

// cleanup will reduce memory footprint of pool.
func (p *pool) cleanup() {
	newList := make([]peer.ID, 0, p.activeCount)
	for _, peerID := range p.peersList {
		status := p.statuses[peerID]
		switch status {
		case active, cooldown:
			newList = append(newList, peerID)
		case removed:
			delete(p.statuses, peerID)
			delete(p.stats, peerID)
		}
	}
	p.peersList = newList
}

// putOnCooldown puts peerID on the base (non-adaptive) cooldown. Retained for callers
// that cool a peer independently of a request outcome.
func (p *pool) putOnCooldown(peerID peer.ID) {
	p.m.Lock()
	defer p.m.Unlock()

	if status, ok := p.statuses[peerID]; ok && status == active {
		p.cooldown.push(peerID, p.params.PeerCooldown)

		p.statuses[peerID] = cooldown
		p.activeCount--
		p.checkHasPeers()
	}
}

func (p *pool) afterCooldown(peerID peer.ID) {
	p.m.Lock()
	defer p.m.Unlock()

	// item could have been already removed by the time afterCooldown is called
	if status, ok := p.statuses[peerID]; !ok || status != cooldown {
		return
	}

	p.statuses[peerID] = active
	p.activeCount++
	p.checkHasPeers()
}

// checkHasPeers will check and indicate if there are peers in the pool.
func (p *pool) checkHasPeers() {
	if p.activeCount > 0 && !p.hasPeer {
		p.hasPeer = true
		close(p.hasPeerCh)
		return
	}

	if p.activeCount == 0 && p.hasPeer {
		p.hasPeerCh = make(chan struct{})
		p.hasPeer = false
	}
}

func (p *pool) len() int {
	p.m.RLock()
	defer p.m.RUnlock()
	return p.activeCount
}
