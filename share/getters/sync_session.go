package getters

import (
	"context"
	"sync/atomic"
)

type getterSession[V any] struct {
	res atomic.Pointer[result[V]]

	gtr getter[V]
	sub *valueSub[V]

	ctx     context.Context
	cancel  context.CancelFunc
	cleanup func()
}

type getter[V any] func(context.Context) (V, error)

type result[V any] struct {
	val V
	err error
}

func newGetSession[V any](gtr getter[V], cleanup func()) *getterSession[V] {
	ctx, cancel := context.WithCancel(context.Background())
	sn := &getterSession[V]{
		gtr:     gtr,
		ctx:     ctx,
		cancel:  cancel,
		cleanup: cleanup,
	}
	sn.sub = newValueSub[V](sn)
	go sn.load()
	return sn
}

func (ss *getterSession[V]) get(ctx context.Context) (V, error) {
	if res := ss.res.Load(); res != nil {
		return res.val, res.err
	}

	if res := ss.sub.sub(ctx); res.err != errSubDuringClean {
		return res.val, res.err
	}

	res := ss.res.Load()
	return res.val, res.err
}

// load executes underlying getter notifying subscribers
// and caching the result
func (ss *getterSession[V]) load() {
	val, err := ss.gtr(ss.ctx)
	res := result[V]{val: val, err: err}
	ss.res.Store(&res)
	ss.sub.pub(res)
}
