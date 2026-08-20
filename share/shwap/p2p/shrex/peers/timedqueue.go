package peers

import (
	"sync"
	"time"

	"github.com/benbjohnson/clock"
	"github.com/libp2p/go-libp2p/core/peer"
)

// timedQueue store items for ttl duration and releases it with calling onPop callback. Each item
// is tracked independently
type timedQueue struct {
	sync.Mutex
	items []item

	// ttl is the amount of time each item exist in the timedQueue
	ttl   time.Duration
	clock clock.Clock
	after *clock.Timer
	// onPop will be called on item peer.ID after it is released
	onPop func(peer.ID)
}

type item struct {
	peer.ID
	createdAt time.Time
}

func newTimedQueue(ttl time.Duration, onPop func(peer.ID)) *timedQueue {
	return &timedQueue{
		items: make([]item, 0),
		clock: clock.New(),
		ttl:   ttl,
		onPop: onPop,
	}
}

// releaseExpired releases all expired items.
func (q *timedQueue) releaseExpired() {
	q.Lock()
	expired := q.releaseUnsafe()
	q.Unlock()

	for _, peerID := range expired {
		q.onPop(peerID)
	}
}

func (q *timedQueue) releaseUnsafe() []peer.ID {
	if len(q.items) == 0 {
		return nil
	}

	var expired []peer.ID
	for _, next := range q.items {
		timeIn := q.clock.Since(next.createdAt)
		if timeIn < q.ttl {
			// item is not expired yet, create a timer that will call releaseExpired
			q.after.Stop()
			q.after = q.clock.AfterFunc(q.ttl-timeIn, q.releaseExpired)
			break
		}

		// item is expired
		expired = append(expired, next.ID)
	}

	if len(expired) > 0 {
		copy(q.items, q.items[len(expired):])
		q.items = q.items[:len(q.items)-len(expired)]
	}
	return expired
}

func (q *timedQueue) push(peerID peer.ID) {
	q.Lock()
	defer q.Unlock()

	q.items = append(q.items, item{
		ID:        peerID,
		createdAt: q.clock.Now(),
	})

	// if it is the first item in queue, create a timer to call releaseExpired after its expiration
	if len(q.items) == 1 {
		q.after = q.clock.AfterFunc(q.ttl, q.releaseExpired)
	}
}

func (q *timedQueue) len() int {
	q.Lock()
	defer q.Unlock()
	return len(q.items)
}
