package getters

import (
	"context"
	"sync/atomic"
)

type getterSession[V any] struct {
	bkts *sessionBuckets[V]

	key string
	res result[V]

	gtr getter[V]
	calls atomic.Uint32

	ctx     context.Context
	cancel  context.CancelFunc
}

type getter[V any] func(context.Context) (V, error)

type result[V any] struct {
	val V
	err error
}

func newGetSession[V any](key string, gtr getter[V], bkts *sessionBuckets[V]) *getterSession[V] {
	ctx, cancel := context.WithCancel(context.Background())
	sn := &getterSession[V]{
		bkts: bkts,
		key: key,
		gtr:     gtr,
		ctx:     ctx,
		cancel:  cancel,
	}
	go sn.execGetter()
	return sn
}

func (ss *getterSession[V]) get(ctx context.Context) (V, error) {
	ss.calls.Add(1) // count calls
	select {
	case <-ss.ctx.Done():
		// we are done getting, return the result
		return ss.res.val, ss.res.err
	case <-ctx.Done():
		// if all the callers cancel
		// cancel getter
		if ss.calls.Load() == 0 {
			ss.cancel()
		}

		var zero V
		return zero, ctx.Err()
	}
}

// execGetter executes underlying getter notifying subscribers
// and caching the result
func (ss *getterSession[V]) execGetter() {
	defer ss.bkts.rmSession(ss.key) // cleanup
	val, err := ss.gtr(ss.ctx) // get
	if val == nil && err == context.Canceled {
		return
	}

	ss.res = result[V]{val: val, err: err} // cache
	ss.cancel() // notify
}
