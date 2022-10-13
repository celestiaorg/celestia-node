package p2p

import (
	"context"
	"sync"

	"github.com/libp2p/go-libp2p-core/event"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
)

type peerTracker struct {
	host         host.Host
	pLk          sync.RWMutex
	notAvailable map[peer.ID]*peerStat
	stats        []*peerStat
}

func newPeerTracker(h host.Host) *peerTracker {
	return &peerTracker{host: h, notAvailable: make(map[peer.ID]*peerStat), stats: make([]*peerStat, 0)}
}

func (p *peerTracker) findPeers(ctx context.Context) {
	// store peers that has been already connected
	for _, peer := range p.host.Peerstore().Peers() {
		p.connected(peer)
	}

	subs, err := p.host.EventBus().Subscribe(&event.EvtPeerConnectednessChanged{})
	if err != nil {
		log.Errorw("subscribing to EvtPeerConnectednessChanged", "err", err)
		return
	}
	for {
		select {
		case <-ctx.Done():
			err = subs.Close()
			if err != nil {
				log.Errorw("closing subscription", "err", err)
			}
			return
		case subscription := <-subs.Out():
			ev := subscription.(event.EvtPeerConnectednessChanged)
			switch ev.Connectedness {
			case network.Connected:
				p.connected(ev.Peer)
			case network.NotConnected:
				p.disconnected(ev.Peer)
			}
		}
	}
}

func (p *peerTracker) connected(pID peer.ID) {
	if p.host.ID() == pID {
		return
	}
	p.pLk.Lock()
	defer p.pLk.Unlock()
	stats, ok := p.notAvailable[pID]
	if !ok {
		stats = &peerStat{peerID: pID}
	} else {
		delete(p.notAvailable, pID)
	}
	p.stats = append(p.stats, stats)
}

func (p *peerTracker) disconnected(pID peer.ID) {
	p.pLk.Lock()
	defer p.pLk.Unlock()
	stats, ok := p.findPeer(pID)
	if ok {
		p.notAvailable[pID] = stats
	}
}

func (p *peerTracker) findPeer(pID peer.ID) (stats *peerStat, ok bool) {
	for _, stat := range p.stats {
		if stat.peerID == pID {
			stats = stat
			ok = true
			break
		}
	}
	return
}

func (p *peerTracker) peers() []*peerStat {
	p.pLk.RLock()
	defer p.pLk.RUnlock()
	return p.stats
}
