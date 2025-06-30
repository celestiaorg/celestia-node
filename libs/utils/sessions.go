package utils //nolint:revive // generic package name is acceptable for utilities

import (
	"context"
	"sync"
)

// Sessions manages concurrent sessions for the specified key.
// It ensures only one session can proceed for each key, avoiding duplicate efforts.
// If a session is already active for the given key, it waits until the session completes or
// context error occurs.
type Sessions struct {
	active sync.Map
}

func NewSessions() *Sessions {
	return &Sessions{}
}

// StartSession attempts to start a new session for the given key. It provides a release function
// to clean up the session lock for this key, once the session is complete.
func (s *Sessions) StartSession(
	ctx context.Context,
	key any,
) (endSession func(), err error) {
	// Attempt to load or initialize a channel to track the sampling session for this height
	lockChan, alreadyActive := s.active.LoadOrStore(key, make(chan struct{}))
	if alreadyActive {
		// If a session is already active, wait for it to complete
		select {
		case <-lockChan.(chan struct{}):
		case <-ctx.Done():
			return func() {}, ctx.Err()
		}
		// previous session has completed, try to obtain the lock for this session
		return s.StartSession(ctx, key)
	}

	// Provide a function to release the lock once session is complete
	releaseLock := func() {
		close(lockChan.(chan struct{}))
		s.active.Delete(key)
	}
	return releaseLock, nil
}
