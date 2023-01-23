package peers

import (
	"context"
	"sync"

	"github.com/libp2p/go-libp2p/core/peer"
)

type pool struct {
	m          *sync.Mutex
	peersList  []peer.ID
	alive      map[peer.ID]bool
	aliveCount int
	next       int

	cleanupEnabled bool
	hasPeer        bool
	hasPeerCh      chan struct{}
}

func newPool() *pool {
	return &pool{
		m:         new(sync.Mutex),
		peersList: make([]peer.ID, 0),
		alive:     make(map[peer.ID]bool),
		hasPeerCh: make(chan struct{}),
	}
}

func (p *pool) tryGet() (peer.ID, bool) {
	p.m.Lock()
	defer p.m.Unlock()

	if p.aliveCount == 0 {
		return "", false
	}

	for {
		peerID := p.peersList[p.next]
		if p.next++; p.next == len(p.peersList) {
			p.next = 0
		}
		if alive := p.alive[peerID]; alive {
			return peerID, true
		}
	}
}

func (p *pool) waitNext(ctx context.Context) <-chan peer.ID {
	peerCh := make(chan peer.ID, 1)
	go func() {
		for {
			select {
			case <-p.hasPeerCh:
				if peerID, ok := p.tryGet(); ok {
					peerCh <- peerID
					return
				}
			case <-ctx.Done():
				close(peerCh)
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
		alive, ok := p.alive[peerID]
		if !ok {
			p.peersList = append(p.peersList, peerID)
		}

		if !ok || !alive {
			p.alive[peerID] = true
			p.aliveCount++
		}
	}
	p.checkHasPeers()
}

func (p *pool) remove(peers ...peer.ID) {
	p.m.Lock()
	defer p.m.Unlock()

	for _, peerID := range peers {
		if alive, ok := p.alive[peerID]; ok && alive {
			p.alive[peerID] = false
			p.aliveCount--
		}
	}

	if len(p.peersList) > p.aliveCount*2 && p.cleanupEnabled {
		p.cleanup()
	}
	p.checkHasPeers()
}

func (p *pool) cleanup() {
	newList := make([]peer.ID, 0, p.aliveCount)
	for idx, peerID := range p.peersList {
		alive := p.alive[peerID]
		if alive {
			newList = append(newList, peerID)
		} else {
			delete(p.alive, peerID)
		}

		if idx == p.next {
			// if peer is not alive and no more alive peers left in list point to first peer
			if !alive && len(newList) >= p.aliveCount {
				p.next = 0
				continue
			}
			p.next = len(newList)
		}
	}
	p.peersList = newList
}

func (p *pool) checkHasPeers() {
	if p.aliveCount > 0 && !p.hasPeer {
		p.hasPeer = true
		close(p.hasPeerCh)
		return
	}

	if p.aliveCount == 0 && p.hasPeer {
		p.hasPeerCh = make(chan struct{})
		p.hasPeer = false
	}
}
