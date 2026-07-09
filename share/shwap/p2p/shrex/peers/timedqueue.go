package peers

import (
	"sync"
	"time"

	"github.com/benbjohnson/clock"
	"github.com/libp2p/go-libp2p/core/peer"
)

// timedQueue stores items for a per-item ttl and releases each by calling onPop once
// its ttl elapses. Items may have different ttls (see adaptive cooldown, ADR-014), so
// the queue is not strictly FIFO by expiry; the release timer always targets the
// earliest outstanding release time.
type timedQueue struct {
	sync.Mutex
	items []item

	// defaultTTL is used by callers that push without an explicit ttl.
	defaultTTL time.Duration
	clock      clock.Clock
	after      *clock.Timer
	// onPop will be called on item peer.ID after it is released
	onPop func(peer.ID)
}

type item struct {
	peer.ID
	releaseAt time.Time
}

func newTimedQueue(defaultTTL time.Duration, onPop func(peer.ID)) *timedQueue {
	return &timedQueue{
		items:      make([]item, 0),
		clock:      clock.New(),
		defaultTTL: defaultTTL,
		onPop:      onPop,
	}
}

// releaseExpired will release all expired items
func (q *timedQueue) releaseExpired() {
	q.Lock()
	defer q.Unlock()
	q.releaseUnsafe()
}

func (q *timedQueue) releaseUnsafe() {
	if len(q.items) == 0 {
		return
	}

	now := q.clock.Now()
	kept := q.items[:0]
	for _, next := range q.items {
		if next.releaseAt.After(now) {
			// not expired yet, keep it
			kept = append(kept, next)
			continue
		}
		q.onPop(next.ID)
	}
	q.items = kept
	q.reschedule(now)
}

// reschedule (re)arms the release timer to fire at the earliest outstanding release
// time. Must be called with the lock held.
func (q *timedQueue) reschedule(now time.Time) {
	if q.after != nil {
		q.after.Stop()
		q.after = nil
	}
	if len(q.items) == 0 {
		return
	}

	earliest := q.items[0].releaseAt
	for _, it := range q.items[1:] {
		if it.releaseAt.Before(earliest) {
			earliest = it.releaseAt
		}
	}

	d := earliest.Sub(now)
	if d < 0 {
		d = 0
	}
	q.after = q.clock.AfterFunc(d, q.releaseExpired)
}

// push schedules peerID for release after ttl. If ttl <= 0 the queue's defaultTTL is used.
func (q *timedQueue) push(peerID peer.ID, ttl time.Duration) {
	q.Lock()
	defer q.Unlock()

	if ttl <= 0 {
		ttl = q.defaultTTL
	}
	now := q.clock.Now()
	q.items = append(q.items, item{
		ID:        peerID,
		releaseAt: now.Add(ttl),
	})
	q.reschedule(now)
}

func (q *timedQueue) len() int {
	q.Lock()
	defer q.Unlock()
	return len(q.items)
}

// releaseAt returns the scheduled release time for peerID and whether it is currently
// queued. Used for diagnostics to report how long a peer remains on cooldown.
func (q *timedQueue) releaseAt(peerID peer.ID) (time.Time, bool) {
	q.Lock()
	defer q.Unlock()
	for _, it := range q.items {
		if it.ID == peerID {
			return it.releaseAt, true
		}
	}
	return time.Time{}, false
}
