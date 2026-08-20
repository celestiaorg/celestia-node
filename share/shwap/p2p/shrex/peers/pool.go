package peers

import (
	"context"
	"math/rand/v2"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
)

const defaultCleanupThreshold = 2

// pool stores peers and provides methods for throughput-based access.
type pool struct {
	m           sync.RWMutex
	peersList   []peer.ID
	statuses    map[peer.ID]status
	cooldown    *timedQueue
	activeCount int

	// stats ranks peers by observed throughput and is shared with all other pools of the
	// same Manager
	stats *peerStats

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

// newPool returns new empty pool.
func newPool(peerCooldownTime time.Duration, stats *peerStats) *pool {
	p := &pool{
		peersList:        make([]peer.ID, 0),
		statuses:         make(map[peer.ID]status),
		stats:            stats,
		hasPeerCh:        make(chan struct{}),
		cleanupThreshold: defaultCleanupThreshold,
	}
	p.cooldown = newTimedQueue(peerCooldownTime, p.afterCooldown)
	return p
}

// tryGet returns peer along with bool flag indicating success of operation.
// Peers are selected by power-of-two-choices: two random active peers are sampled and the one
// with the higher throughput score wins. Unmeasured peers get priority so every peer can establish
// a score. Sampling instead of always taking the best score limits herding onto a single peer.
func (p *pool) tryGet() (peer.ID, bool) {
	p.m.Lock()
	defer p.m.Unlock()

	if p.activeCount == 0 {
		return "", false
	}

	first, idx, ok := p.activeFrom(rand.IntN(len(p.peersList))) //nolint:gosec
	if !ok {
		return "", false
	}
	if p.activeCount == 1 {
		return first, true
	}

	second, _, ok := p.activeFrom(rand.IntN(len(p.peersList))) //nolint:gosec
	if second == first {
		// both draws landed on the same peer, take the next active one instead
		second, _, ok = p.activeFrom(idx + 1)
	}
	if !ok {
		return first, true
	}

	return p.stats.selectPeer(first, second), true
}

// activeFrom returns the first active peer at or after idx, wrapping around the peers list.
func (p *pool) activeFrom(idx int) (peer.ID, int, bool) {
	for i := range p.peersList {
		j := (idx + i) % len(p.peersList)
		if p.statuses[p.peersList[j]] == active {
			return p.peersList[j], j, true
		}
	}
	return "", 0, false
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
		}
	}
	p.peersList = newList
}

func (p *pool) putOnCooldown(peerID peer.ID) {
	p.m.Lock()
	defer p.m.Unlock()

	if status, ok := p.statuses[peerID]; ok && status == active {
		p.cooldown.push(peerID)

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
