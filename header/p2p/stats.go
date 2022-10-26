package p2p

import (
	"container/heap"
	"sync"
	"sync/atomic"

	"github.com/libp2p/go-libp2p-core/peer"
)

// peerStat represents peer's average statistic.
type peerStat struct {
	pLk    sync.RWMutex
	peerID peer.ID
	// score is average speed per 1 request
	peerScore float32
}

// updateStats recalculates peer.score by averaging the last score
func (p *peerStat) updateStats(amount uint64, time uint64) {
	p.pLk.Lock()
	defer p.pLk.Unlock()
	var averageSpeed float32
	if time != 0 {
		averageSpeed = float32(amount / time)
	}
	if p.peerScore == 0.0 {
		p.peerScore = averageSpeed
		return
	}
	p.peerScore = (p.peerScore + averageSpeed) / 2
}

// score peer latest score in the queue
func (p *peerStat) score() float32 {
	p.pLk.RLock()
	defer p.pLk.RUnlock()
	return p.peerScore
}

type peerStats []*peerStat

func newPeerStats() peerStats {
	ps := make(peerStats, 0)
	heap.Init(&ps)
	return ps
}

// peerQueue wraps peerStats
type peerQueue struct {
	stats   peerStats
	statsLk sync.RWMutex

	havePeer chan struct{}
	wantPeer atomic.Bool
}

func newPeerQueue(stats []*peerStat) *peerQueue {
	pq := &peerQueue{
		stats: newPeerStats(),
	}
	for _, stat := range stats {
		pq.push(stat)
	}
	return pq
}

// calculateBestPeer pops the peer with the biggest score.
// in case if there are no peer available in current session, it blocks until
// a peer will be pushed in.
func (p *peerQueue) calculateBestPeer() *peerStat {
	if p.len() == 0 {
		p.wantPeer.Store(true)
		defer p.wantPeer.Store(false)
		<-p.havePeer
	}
	return p.pop()
}

// push adds the peer to the queue.
func (p *peerQueue) push(stat *peerStat) {
	p.statsLk.Lock()
	defer p.statsLk.Unlock()
	heap.Push(&p.stats, stat)
	// notify that peer is available in the queue, so it can be popped out
	if p.wantPeer.Load() {
		p.havePeer <- struct{}{}
	}
}

// pop removes the peer with the biggest score from the queue.
func (p *peerQueue) pop() *peerStat {
	p.statsLk.Lock()
	defer p.statsLk.Unlock()
	return heap.Pop(&p.stats).(*peerStat)
}

func (p *peerQueue) len() int {
	p.statsLk.Lock()
	defer p.statsLk.Unlock()
	return p.stats.Len()
}

/*
Further methods make peerStats implements heap.Interface:

	type Interface interface {
		sort.Interface
		Push(x any)
		Pop() any
	}
*/
func (pq peerStats) Len() int { return len(pq) }

func (pq peerStats) Less(i, j int) bool {
	return pq[i].score() > pq[j].score()
}

func (pq peerStats) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
}

func (pq *peerStats) Push(x any) {
	item := x.(*peerStat)
	*pq = append(*pq, item)
}

func (pq *peerStats) Pop() any {
	old := *pq
	n := len(old)
	item := old[n-1]
	old[n-1] = nil
	*pq = old[0 : n-1]
	return item
}
