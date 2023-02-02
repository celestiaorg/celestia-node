package peers

import (
	"context"
	"sync"

	"github.com/libp2p/go-libp2p/core/peer"
)

// pool stores peers and provides methods for simple round-robin access.
type pool struct {
	m           sync.Mutex
	peersList   []peer.ID
	active      map[peer.ID]bool
	activeCount int
	next        int

	hasPeer   bool
	hasPeerCh chan struct{}

	cleanupDisabled bool
}

// newPool creates new pool
func newPool() *pool {
	return &pool{
		peersList: make([]peer.ID, 0),
		active:    make(map[peer.ID]bool),
		hasPeerCh: make(chan struct{}),
	}
}

// tryGet returns peer along with bool flag indicating success of operation.
func (p *pool) tryGet() (peer.ID, bool) {
	p.m.Lock()
	defer p.m.Unlock()

	if p.activeCount == 0 {
		return "", false
	}

	start := p.next
	for {
		peerID := p.peersList[p.next]

		p.next++
		if p.next == len(p.peersList) {
			p.next = 0
		}

		if alive := p.active[peerID]; alive {
			return peerID, true
		}

		// full circle passed
		if p.next == start {
			return "", false
		}
	}
}

// getNext sends a peer to the returned channel when it becomes available.
func (p *pool) getNext(ctx context.Context) <-chan peer.ID {
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
		alive, ok := p.active[peerID]
		if !ok {
			p.peersList = append(p.peersList, peerID)
		}

		if !ok || !alive {
			p.active[peerID] = true
			p.activeCount++
		}
	}
	p.checkHasPeers()
}

func (p *pool) remove(peers ...peer.ID) {
	p.m.Lock()
	defer p.m.Unlock()

	for _, peerID := range peers {
		if alive, ok := p.active[peerID]; ok && alive {
			p.active[peerID] = false
			p.activeCount--
		}
	}

	// do cleanup if too much garbage
	if len(p.peersList) > p.activeCount*2 && !p.cleanupDisabled {
		p.cleanup()
	}
	p.checkHasPeers()
}

// cleanup will reduce memory footprint of pool.
func (p *pool) cleanup() {
	newList := make([]peer.ID, 0, p.activeCount)
	for idx, peerID := range p.peersList {
		alive := p.active[peerID]
		if alive {
			newList = append(newList, peerID)
		} else {
			delete(p.active, peerID)
		}

		if idx == p.next {
			// if peer is not active and no more active peers left in list point to first peer
			if !alive && len(newList) >= p.activeCount {
				p.next = 0
				continue
			}
			p.next = len(newList)
		}
	}
	p.peersList = newList
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
