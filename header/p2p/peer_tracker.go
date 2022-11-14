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
	sync.RWMutex
	connectedPeers map[peer.ID]*peerStat
	// we cache the peer once they disconnect,
	// so we can guarantee that peerQueue will only contain active peers
	disconnectedPeers map[peer.ID]*peerStat

	host host.Host
}

func newPeerTracker(h host.Host) *peerTracker {
	return &peerTracker{
		disconnectedPeers: make(map[peer.ID]*peerStat),
		connectedPeers:    make(map[peer.ID]*peerStat),
		host:              h,
	}
}

func (p *peerTracker) track(ctx context.Context) {
	// store peers that have been already connected
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
		// check if connection is short-termed and skip this peer
		if c.Stat().Transient {
			return
		}
	}
	p.Lock()
	defer p.Unlock()
	// additional check in p.connectedPeers should be done,
	// because libp2p does not emit multiple Connected events per 1 peer
	stats, ok := p.disconnectedPeers[pID]
	if !ok {
		stats = &peerStat{peerID: pID}
	} else {
		delete(p.disconnectedPeers, pID)
	}
	p.connectedPeers[pID] = stats
}

func (p *peerTracker) disconnected(pID peer.ID) {
	p.Lock()
	defer p.Unlock()
	stats, ok := p.connectedPeers[pID]
	if !ok {
		return
	}
	p.disconnectedPeers[pID] = stats
	delete(p.connectedPeers, pID)
}

func (p *peerTracker) peers() []*peerStat {
	p.RLock()
	defer p.RUnlock()
	peers := make([]*peerStat, 0, len(p.connectedPeers))
	for _, stat := range p.connectedPeers {
		peers = append(peers, stat)
	}
	return peers
}
