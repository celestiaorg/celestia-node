package peers

import (
	"slices"
	"time"

	"github.com/benbjohnson/clock"
	"github.com/libp2p/go-libp2p/core/peer"
)

// timedQueue stores items for ttl. The owner must hold its lock for all queue access,
// including expiry from onTimer.
type timedQueue struct {
	items   []item
	ttl     time.Duration
	clock   clock.Clock
	after   *clock.Timer
	onTimer func()
}

type item struct {
	peer.ID
	createdAt time.Time
}

func newTimedQueue(ttl time.Duration, onTimer func()) *timedQueue {
	return &timedQueue{
		items:   make([]item, 0),
		clock:   clock.New(),
		ttl:     ttl,
		onTimer: onTimer,
	}
}

// releaseExpired removes expired items and calls onPop under the owner's lock.
func (q *timedQueue) releaseExpired(onPop func(peer.ID)) {
	n := 0
	for _, next := range q.items {
		if q.clock.Since(next.createdAt) < q.ttl {
			break
		}
		onPop(next.ID)
		n++
	}
	q.items = slices.Delete(q.items, 0, n)
	q.schedule()
}

func (q *timedQueue) push(peerID peer.ID) {
	q.items = append(q.items, item{
		ID:        peerID,
		createdAt: q.clock.Now(),
	})
	if len(q.items) == 1 {
		q.schedule()
	}
}

func (q *timedQueue) remove(peerID peer.ID) {
	for i, entry := range q.items {
		if entry.ID == peerID {
			q.items = slices.Delete(q.items, i, i+1)
			if i == 0 {
				q.schedule()
			}
			return
		}
	}
}

func (q *timedQueue) schedule() {
	if q.after != nil {
		q.after.Stop()
		q.after = nil
	}
	if len(q.items) > 0 {
		q.after = q.clock.AfterFunc(q.ttl-q.clock.Since(q.items[0].createdAt), q.onTimer)
	}
}

func (q *timedQueue) len() int {
	return len(q.items)
}
