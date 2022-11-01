package p2p

import (
	"container/heap"
	"sync"
	"sync/atomic"

	"github.com/libp2p/go-libp2p-core/peer"
)

// peerStat represents peer's average statistic.
type peerStat struct {
	sync.RWMutex
	peerID peer.ID
	// score is average speed per 1 request
	peerScore float32
}

// updateStats recalculates peer.score by averaging the last score
// updateStats takes total amount of bytes that were requested from the peer
// and total request duration(in milliseconds). The final score is calculated
// by dividing amount on time, so the result score will represent how many bytes
// were retrieved during 1 millisecond. Then this value will be averaged relative to the
// previous peerScore.
func (p *peerStat) updateStats(amount uint64, time uint64) {
	p.Lock()
	defer p.Unlock()
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
	p.RLock()
	defer p.RUnlock()
	return p.peerScore
}

/*
peerStats implements heap.Interface, so we can be sure that we are getting the peer
with the highest score, each time we call Pop.

Methods/Interfaces that peerStats should implement:
  - sort.Interface
  - Push(x any)
  - Pop() any
*/
type peerStats []*peerStat

func newPeerStats() peerStats {
	ps := make(peerStats, 0)
	heap.Init(&ps)
	return ps
}

func (ps peerStats) Len() int { return len(ps) }

func (ps peerStats) Less(i, j int) bool {
	return ps[i].score() > ps[j].score()
}

func (ps peerStats) Swap(i, j int) {
	ps[i], ps[j] = ps[j], ps[i]
}

func (ps *peerStats) Push(x any) {
	item := x.(*peerStat)
	*ps = append(*ps, item)
}

func (ps *peerStats) Pop() any {
	old := *ps
	n := len(old)
	item := old[n-1]
	old[n-1] = nil
	*ps = old[0 : n-1]
	return item
}

// peerQueue wraps peerStats and guards it with a mutex.
type peerQueue struct {
	statsLk sync.RWMutex
	stats   peerStats

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
