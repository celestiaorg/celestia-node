package getters

import (
	"context"
	"errors"
	"sync"
)

var errSubDuringClean = errors.New("sub during cleanup")

type valueSub[V any] struct {
	sn *getterSession[V]

	reqsMu sync.Mutex
	reqs   map[chan result[V]]struct{}
}

func newValueSub[V any](sn *getterSession[V]) *valueSub[V] {
	return &valueSub[V]{
		sn:   sn,
		reqs: make(map[chan result[V]]struct{}),
	}
}

func (vs *valueSub[V]) sub(ctx context.Context) result[V] {
	req := vs.newReq()
	select {
	case res := <-req:
		return res
	case <-ctx.Done():
		vs.reqsMu.Lock()
		vs.rmReq(req)
		vs.reqsMu.Unlock()
		return result[V]{err: ctx.Err()}
	case <-vs.sn.ctx.Done():
		return result[V]{err: errSubDuringClean}
	}
}

func (vs *valueSub[V]) pub(res result[V]) {
	vs.reqsMu.Lock()
	for req := range vs.reqs {
		req <- res // never blocks
		vs.rmReq(req)
	}
	vs.reqsMu.Unlock()
}

func (vs *valueSub[V]) newReq() chan result[V] {
	req := make(chan result[V], 1)
	vs.reqsMu.Lock()
	vs.reqs[req] = struct{}{}
	vs.reqsMu.Unlock()
	return req
}

func (vs *valueSub[V]) rmReq(req chan result[V]) {
	delete(vs.reqs, req)
	if len(vs.reqs) == 0 {
		// cancel if no one waits for the data
		vs.sn.cancel()
		vs.sn.cleanup()
	}
}
