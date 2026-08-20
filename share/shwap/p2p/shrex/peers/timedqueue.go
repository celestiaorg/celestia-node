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
	// onPop will be called on an item after it is released
	onPop     func(peer.ID, uint64)
	nextToken uint64
}

type item struct {
	peer.ID
	createdAt time.Time
	token     uint64
}

func newTimedQueue(ttl time.Duration, onPop func(peer.ID, uint64)) *timedQueue {
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

	for _, item := range expired {
		q.onPop(item.ID, item.token)
	}
}

func (q *timedQueue) releaseUnsafe() []item {
	if len(q.items) == 0 {
		return nil
	}

	var expired []item
	for _, next := range q.items {
		timeIn := q.clock.Since(next.createdAt)
		if timeIn < q.ttl {
			// item is not expired yet, create a timer that will call releaseExpired
			q.after.Stop()
			q.after = q.clock.AfterFunc(q.ttl-timeIn, q.releaseExpired)
			break
		}

		// item is expired
		expired = append(expired, next)
	}

	if len(expired) > 0 {
		copy(q.items, q.items[len(expired):])
		q.items = q.items[:len(q.items)-len(expired)]
	}
	return expired
}

func (q *timedQueue) push(peerID peer.ID) uint64 {
	q.Lock()
	defer q.Unlock()

	q.nextToken++
	q.items = append(q.items, item{
		ID:        peerID,
		createdAt: q.clock.Now(),
		token:     q.nextToken,
	})

	// if it is the first item in queue, create a timer to call releaseExpired after its expiration
	if len(q.items) == 1 {
		q.after = q.clock.AfterFunc(q.ttl, q.releaseExpired)
	}
	return q.nextToken
}

func (q *timedQueue) len() int {
	q.Lock()
	defer q.Unlock()
	return len(q.items)
}
