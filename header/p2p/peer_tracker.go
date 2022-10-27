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
	host host.Host

	peerLk         sync.RWMutex
	connectedPeers map[peer.ID]*peerStat
	// we should store peer to cache when it will be disconnected,
	// so we can guarantee that peerQueue will return only active peer
	disconnectedPeers map[peer.ID]*peerStat
}

func newPeerTracker(h host.Host) *peerTracker {
	return &peerTracker{
		host:              h,
		disconnectedPeers: make(map[peer.ID]*peerStat),
		connectedPeers:    make(map[peer.ID]*peerStat),
	}
}

func (p *peerTracker) track(ctx context.Context) {
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
	for _, c := range p.host.Network().ConnsToPeer(pID) {
		if c.Stat().Transient {
			return
		}
	}
	p.peerLk.Lock()
	defer p.peerLk.Unlock()
	stats, ok := p.disconnectedPeers[pID]
	if !ok {
		stats = &peerStat{peerID: pID}
	} else {
		delete(p.disconnectedPeers, pID)
	}
	p.connectedPeers[pID] = stats
}

func (p *peerTracker) disconnected(pID peer.ID) {
	p.peerLk.Lock()
	defer p.peerLk.Unlock()
	stats, ok := p.connectedPeers[pID]
	if !ok {
		return
	}
	p.disconnectedPeers[pID] = stats
	delete(p.connectedPeers, pID)
}

func (p *peerTracker) peers() []*peerStat {
	p.peerLk.RLock()
	defer p.peerLk.RUnlock()
	peers := make([]*peerStat, 0, len(p.connectedPeers))
	for _, stat := range p.connectedPeers {
		peers = append(peers, stat)
	}
	return peers
}
