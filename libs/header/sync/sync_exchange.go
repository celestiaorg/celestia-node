package sync

import (
	"context"
	"sync"

	"github.com/celestiaorg/celestia-node/libs/header"
)

type syncExchange[H header.Header] struct {
	header.Exchange[H]

	headLk  sync.RWMutex
	headErr error
	head    H
}

func (se *syncExchange[H]) Head(ctx context.Context) (H, error) {
	// the lock construction here ensures only one routine calling Head at a time
	// while others wait via Rlock
	if !se.headLk.TryLock() {
		se.headLk.RLock()
		defer se.headLk.RUnlock()
		return se.head, se.headErr
	}
	defer se.headLk.Unlock()

	se.head, se.headErr = se.Exchange.Head(ctx)
	return se.head, se.headErr
}
