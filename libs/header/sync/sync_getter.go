package sync

import (
	"context"
	"sync"

	"github.com/celestiaorg/celestia-node/libs/header"
)

// syncGetter is a Getter wrapper that ensure only one Head call happens at the time
type syncGetter[H header.Header] struct {
	header.Getter[H]

	headLk  sync.RWMutex
	headErr error
	head    H
}

func (se *syncGetter[H]) Head(ctx context.Context) (H, error) {
	// the lock construction here ensures only one routine calling Head at a time
	// while others wait via Rlock
	if !se.headLk.TryLock() {
		se.headLk.RLock()
		defer se.headLk.RUnlock()
		return se.head, se.headErr
	}
	defer se.headLk.Unlock()

	se.head, se.headErr = se.Getter.Head(ctx)
	return se.head, se.headErr
}
