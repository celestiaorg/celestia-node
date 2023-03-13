package sync

import (
	"context"
	"sync/atomic"

	"github.com/celestiaorg/celestia-node/libs/header"
)

// syncStore is a Store wrapper that provides synchronization over writes and reads
// for Head of underlying Store. Useful for Stores that do not guarantee synchrony between Append
// and Head method.
type syncStore[H header.Header] struct {
	header.Store[H]

	head atomic.Pointer[H]
}

func (s *syncStore[H]) Head(ctx context.Context) (H, error) {
	if headPtr := s.head.Load(); headPtr != nil {
		return *headPtr, nil
	}

	storeHead, err := s.Store.Head(ctx)
	if err != nil {
		return storeHead, err
	}

	s.head.Store(&storeHead)
	return storeHead, nil
}

func (s *syncStore[H]) Append(ctx context.Context, headers ...H) error {
	if err := s.Store.Append(ctx, headers...); err != nil {
		return err
	}

	s.head.Store(&headers[len(headers)-1])
	return nil
}
