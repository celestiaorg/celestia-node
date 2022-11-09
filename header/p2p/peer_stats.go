package p2p

import (
	"container/heap"
	"context"
	"sync"

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

// peerStats implements heap.Interface, so we can be sure that we are getting the peer
// with the highest score, each time we call Pop.
type peerStats []*peerStat

func newPeerStats() peerStats {
	ps := make(peerStats, 0)
	heap.Init(&ps)
	return ps
}

func (ps peerStats) Len() int { return len(ps) }

// Less compares two peerScores.
// Less is used by heap.Interface to build the queue in a decreasing order.
func (ps peerStats) Less(i, j int) bool {
	return ps[i].score() > ps[j].score()
}

func (ps peerStats) Swap(i, j int) {
	ps[i], ps[j] = ps[j], ps[i]
}

// Push adds peerStat to the queue.
func (ps *peerStats) Push(x any) {
	item := x.(*peerStat)
	*ps = append(*ps, item)
}

// Pop returns the peer with the highest score from the queue.
func (ps *peerStats) Pop() any {
	old := *ps
	n := len(old)
	item := old[n-1]
	old[n-1] = nil
	*ps = old[:n-1]
	return item
}

// peerQueue wraps peerStats and guards it with the mutex.
type peerQueue struct {
	statsLk sync.RWMutex
	stats   peerStats

	havePeer chan struct{}
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
// the peer will be pushed in.
func (p *peerQueue) calculateBestPeer(ctx context.Context) *peerStat {
	p.statsLk.Lock()
	defer p.statsLk.Unlock()
	if p.stats.Len() == 0 {
		select {
		case <-ctx.Done():
			return &peerStat{}
		case <-p.havePeer:
		}
	}
	return p.pop()
}

// push adds the peer to the queue.
func (p *peerQueue) push(stat *peerStat) {
	p.statsLk.Lock()
	defer p.statsLk.Unlock()
	heap.Push(&p.stats, stat)
	// notify that the peer is available in the queue, so it can be popped out
	select {
	case p.havePeer <- struct{}{}:
	default:
	}
}

// pop removes the peer with the biggest score from the queue.
func (p *peerQueue) pop() *peerStat {
	p.statsLk.Lock()
	defer p.statsLk.Unlock()
	return heap.Pop(&p.stats).(*peerStat)
}
