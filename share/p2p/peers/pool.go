package peers

import (
	"context"
	"github.com/libp2p/go-libp2p/core/peer"
	"sync"
	"sync/atomic"
)

type pool struct {
	m          *sync.Mutex
	peersList  []peer.ID
	alive      map[peer.ID]bool
	aliveCount int
	next       int

	hasPeer   *atomic.Bool
	hasPeerCh chan struct{}
}

func newPool() *pool {
	return &pool{
		m:         new(sync.Mutex),
		peersList: make([]peer.ID, 0),
		alive:     make(map[peer.ID]bool),
		hasPeer:   new(atomic.Bool),
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
		peer := p.peersList[p.next]
		if p.next++; p.next == len(p.peersList) {
			p.next = 0
		}
		_, ok := p.alive[peer]
		if ok {
			return peer, true
		}
	}
}

func (p *pool) waitNext(ctx context.Context) <-chan peer.ID {
	peerCh := make(chan peer.ID, 1)
	go func() {
		for {
			select {
			case <-p.hasPeerCh:
				if peer, ok := p.tryGet(); ok {
					peerCh <- peer
					return
				}
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

	for _, peer := range peers {
		alive, ok := p.alive[peer]
		if !ok {
			p.peersList = append(p.peersList, peer)
		}

		if !ok || !alive {
			p.alive[peer] = true
			p.aliveCount++
		}
	}
	p.checkHasPeers()
}

func (p *pool) remove(peers ...peer.ID) {
	p.m.Lock()
	defer p.m.Unlock()

	for _, peer := range peers {
		if alive, ok := p.alive[peer]; ok && alive {
			p.alive[peer] = false
			p.aliveCount--
		}
	}

	if len(p.peersList) > p.aliveCount*2 {
		p.cleanup()
	}
	p.checkHasPeers()
}

func (p *pool) cleanup() {
	newList := make([]peer.ID, 0, p.aliveCount)
	for idx, peer := range p.peersList {
		alive := p.alive[peer]
		if alive {
			newList = append(newList, peer)
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
	if p.aliveCount > 0 && p.hasPeer.CompareAndSwap(false, true) {
		close(p.hasPeerCh)
		return
	}

	if p.aliveCount == 0 && p.hasPeer.Load() {
		p.hasPeerCh = make(chan struct{})
		p.hasPeer.Store(true)
	}
}
