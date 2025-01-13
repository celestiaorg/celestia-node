package utils

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestSessionsSerialExecution verifies that multiple sessions for the same key are executed
// sequentially.
func TestSessionsSerialExecution(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	t.Cleanup(cancel)

	sessions := NewSessions()
	key := "testKey"
	activeCount := atomic.Int32{}
	var wg sync.WaitGroup

	numSessions := 20

	for i := 0; i < numSessions; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			endSession, err := sessions.StartSession(ctx, key)
			require.NoError(t, err)
			old := activeCount.Add(1)
			require.Equal(t, int32(1), old)
			// Simulate some work
			time.Sleep(50 * time.Millisecond)
			old = activeCount.Add(-1)
			require.Equal(t, int32(0), old)
			// Release the session
			endSession()
		}(i)
	}

	wg.Wait()
}

func TestSessionsContextCancellation(t *testing.T) {
	sessions := NewSessions()
	key := "testCancelKey"

	// Start the first session which will hold the lock for a while
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		release, err := sessions.StartSession(ctx, key)
		if err != nil {
			t.Errorf("First session: failed to start: %v", err)
			return
		}

		// Hold the session for 1 second
		time.Sleep(1 * time.Second)
		release()
	}()

	// Give the first goroutine a moment to acquire the session
	time.Sleep(100 * time.Millisecond)

	// Attempt to start a second session with a context that times out before the first session releases
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	t.Cleanup(cancel)

	_, err := sessions.StartSession(ctx, key)
	require.ErrorIs(t, err, context.DeadlineExceeded)

	// Attempt to start a second session with a context that is canceled before the first session
	// releases
	ctx, cancel = context.WithCancel(context.Background())
	cancel()

	_, err = sessions.StartSession(ctx, key)
	require.ErrorIs(t, err, context.Canceled)

	wg.Wait()
}

// TestSessions_ConcurrentDifferentKeys ensures that sessions with different keys run concurrently.
func TestSessions_ConcurrentDifferentKeys(t *testing.T) {
	sessions := NewSessions()
	numKeys := 20
	var wg sync.WaitGroup
	startCh := make(chan struct{})
	activeSessions := atomic.Int32{}
	maxActive := int32(0)

	for i := 0; i < numKeys; i++ {
		wg.Add(1)
		go func(key int) {
			defer wg.Done()
			ctx := context.Background()
			endSession, err := sessions.StartSession(ctx, key)
			require.NoError(t, err)

			active := activeSessions.Add(1)
			if active > maxActive {
				maxActive = active
			}

			// Wait to simulate work
			time.Sleep(100 * time.Millisecond)

			activeSessions.Add(-1)
			endSession()
		}(i)
	}

	// Start all goroutines
	close(startCh)
	wg.Wait()

	if maxActive > int32(numKeys) {
		t.Errorf("Expected %d concurrent active sessions, but got %d", numKeys, maxActive)
	}
}
