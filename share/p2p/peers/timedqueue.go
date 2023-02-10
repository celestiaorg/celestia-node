package peers

import (
	"sync"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/libp2p/go-libp2p/core/peer"
)

type timedQueue struct {
	sync.Mutex
	items []item

	ttl   time.Duration
	clock clockwork.Clock
	after clockwork.Timer
	onPop func(peer.ID)
}

type item struct {
	peer.ID
	createdAt time.Time
}

func newTimedQueue(ttl time.Duration, onPop func(peer.ID)) *timedQueue {
	return &timedQueue{
		items: make([]item, 0),
		clock: clockwork.NewRealClock(),
		ttl:   ttl,
		onPop: onPop,
	}
}

func (q *timedQueue) releaseExpired() {
	q.Lock()
	defer q.Unlock()
	q.releaseUnsafe()
}

func (q *timedQueue) releaseUnsafe() {
	if len(q.items) == 0 {
		return
	}

	var i int
	for _, next := range q.items {
		timeIn := q.clock.Since(next.createdAt)
		if timeIn < q.ttl {
			// item not expired yet, create a timer that will call releaseExpired
			q.after.Stop()
			q.after = q.clock.AfterFunc(q.ttl-timeIn, q.releaseExpired)
			break
		}

		q.onPop(next.ID)
		i++
	}

	if i > 0 {
		copy(q.items, q.items[i:])
		q.items = q.items[:len(q.items)-i]
	}
}

func (q *timedQueue) push(peerID peer.ID) {
	q.Lock()
	defer q.Unlock()

	q.items = append(q.items, item{
		ID:        peerID,
		createdAt: q.clock.Now(),
	})

	// set timer after item is added so releaseExpired after expiration happens
	if len(q.items) == 1 {
		q.after = q.clock.AfterFunc(q.ttl, q.releaseExpired)
	}
}

func (q *timedQueue) len() int {
	q.Lock()
	defer q.Unlock()
	return len(q.items)
}
