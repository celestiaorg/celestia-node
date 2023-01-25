package getters

import (
	"sync"
	"sync/atomic"
)

type sessionBuckets[V any] [256]atomic.Pointer[sessionBucket[V]]

type sessionBucket[V any] struct {
	snsLk sync.RWMutex
	sns   map[string]*getterSession[V]
}

func (sb *sessionBuckets[V]) getSession(key string, gtr getter[V]) *getterSession[V] {
	newFn := func() *getterSession[V] {
		return newGetSession[V](key, gtr, sb)
	}

	return sb.getOrNewSession(key, newFn)
}

func (sb *sessionBuckets[V]) rmSession(key string) {
	bkt := sb.getBucket(key)

	bkt.snsLk.Lock()
	delete(bkt.sns, key)
	bkt.snsLk.Unlock()
}

func (sb *sessionBuckets[V]) getOrNewSession(key string, new func() *getterSession[V]) *getterSession[V] {
	bkt := sb.getBucket(key)

	bkt.snsLk.RLock()
	sn, ok := bkt.sns[key]
	bkt.snsLk.RUnlock()
	if ok {
		return sn
	}

	bkt.snsLk.Lock()
	defer bkt.snsLk.Unlock()
	if sn = bkt.sns[key]; sn != nil {
		return sn
	}

	sn = new()
	bkt.sns[key] = sn
	return sn
}

// Explain why not to remove buckets
func (sb *sessionBuckets[V]) getBucket(key string) *sessionBucket[V] {
	bktIdx := key[0]
	bktPtr := &sb[bktIdx]

	bkt := bktPtr.Load()
	for bkt == nil {
		bkt = &sessionBucket[V]{sns: make(map[string]*getterSession[V])}
		if bktPtr.CompareAndSwap(nil, bkt) {
			break
		}
		bkt = bktPtr.Load()
	}

	return bkt
}
