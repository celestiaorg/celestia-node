package p2p

import (
	"context"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p-core/event"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
)

type peerTracker struct {
	sync.RWMutex

	ctx    context.Context
	cancel context.CancelFunc

	connectedPeers map[peer.ID]*peerStat
	// we cache the peer once they disconnect,
	// so we can guarantee that peerQueue will only contain active peers
	disconnectedPeers map[peer.ID]*peerStat

	host host.Host

	gcPeriod           time.Duration
	maxAwaitingTime    time.Duration
	defaultScore       float32
	maxPeerTrackerSize int

	done chan struct{}
}

func newPeerTracker(
	h host.Host,
	gcPeriod, maxAwaitingTime time.Duration,
	defaultScore float32,
	maxPeerTrackerSize int,
) *peerTracker {
	ctx, cancel := context.WithCancel(context.Background())
	return &peerTracker{
		ctx:                ctx,
		cancel:             cancel,
		disconnectedPeers:  make(map[peer.ID]*peerStat),
		connectedPeers:     make(map[peer.ID]*peerStat),
		host:               h,
		gcPeriod:           gcPeriod,
		maxAwaitingTime:    maxAwaitingTime,
		defaultScore:       defaultScore,
		maxPeerTrackerSize: maxPeerTrackerSize,
		done:               make(chan struct{}, 2),
	}
}

func (p *peerTracker) track() {
	defer func() {
		p.done <- struct{}{}
	}()

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
		case <-p.ctx.Done():
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
	p.Lock()
	defer p.Unlock()
	// skip adding the peer to avoid overfilling of the peerTracker with unused peers if:
	// peerTracker reaches the maxTrackerSize and the size of the connected peers
	// is more than the size of the disconnected peers.
	if len(p.connectedPeers)+len(p.disconnectedPeers) > p.maxPeerTrackerSize &&
		len(p.connectedPeers) > len(p.disconnectedPeers) {
		return
	}

	for _, c := range p.host.Network().ConnsToPeer(pID) {
		// check if connection is short-termed and skip this peer
		if c.Stat().Transient {
			return
		}
	}

	// additional check in p.connectedPeers should be done,
	// because libp2p does not emit multiple Connected events per 1 peer
	stats, ok := p.disconnectedPeers[pID]
	if !ok {
		stats = &peerStat{peerID: pID, peerScore: p.defaultScore}
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
	stats.removedAt = time.Now().Add(p.maxAwaitingTime)
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

// gc goes through connected and disconnected peers once in gcPeriod
// and removes every peer that meets conditions:
// * disconnected peer will be removed if it is being disconnected for more than maxAwaitingTime;
// * connected peer will be removed if it scores less or equal than defaultScore;
func (p *peerTracker) gc() {
	ticker := time.NewTicker(p.gcPeriod)
	for {
		select {
		case <-p.ctx.Done():
			p.done <- struct{}{}
			return
		case <-ticker.C:
			p.Lock()
			// disconnectedPeers will be removed it removedAt < time.Now
			for id, peer := range p.disconnectedPeers {
				if peer.removedAt.Before(time.Now()) {
					delete(p.disconnectedPeers, id)
				}
			}

			// connectedPeers will be removed if their score is less of equal to defaultScore
			for id, peer := range p.connectedPeers {
				if peer.peerScore <= p.defaultScore {
					delete(p.connectedPeers, id)
				}
			}
			p.Unlock()
		}
	}
}

// stop waits until all background routines will be finished.
func (p *peerTracker) stop() {
	p.cancel()

	for i := 0; i < cap(p.done); i++ {
		<-p.done
	}
}
