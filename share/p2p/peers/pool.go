package peers

import (
	"context"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
)

const defaultCleanupThreshold = 2

// pool stores peers and provides methods for simple round-robin access.
type pool struct {
	m           sync.RWMutex
	peersList   []peer.ID
	statuses    map[peer.ID]status
	cooldown    *timedQueue
	activeCount int
	nextIdx     int

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
func newPool(peerCooldownTime time.Duration) *pool {
	p := &pool{
		peersList:        make([]peer.ID, 0),
		statuses:         make(map[peer.ID]status),
		hasPeerCh:        make(chan struct{}),
		cleanupThreshold: defaultCleanupThreshold,
	}
	p.cooldown = newTimedQueue(peerCooldownTime, p.afterCooldown)
	return p
}

// tryGet returns peer along with bool flag indicating success of operation.
func (p *pool) tryGet() (peer.ID, bool) {
	p.m.Lock()
	defer p.m.Unlock()

	if p.activeCount == 0 {
		return "", false
	}

	// if pointer is out of range, point to first element
	if p.nextIdx > len(p.peersList)-1 {
		p.nextIdx = 0
	}

	start := p.nextIdx
	for {
		peerID := p.peersList[p.nextIdx]

		p.nextIdx++
		if p.nextIdx == len(p.peersList) {
			p.nextIdx = 0
		}

		if p.statuses[peerID] == active {
			return peerID, true
		}

		// full circle passed
		if p.nextIdx == start {
			return "", false
		}
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

			select {
			case <-p.hasPeerCh:
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
