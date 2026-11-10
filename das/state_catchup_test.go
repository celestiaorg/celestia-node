package das

import (
	"context"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCoordinatorStateWaitCatchUpConcurrentReset(t *testing.T) {
	s := newCoordinatorState(DefaultParameters())
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var waiters sync.WaitGroup
	for range 8 {
		waiters.Go(func() {
			for ctx.Err() == nil {
				if s.waitCatchUp(ctx) != nil {
					return
				}
				runtime.Gosched()
			}
		})
	}
	// Only the coordinator changes sampling state; callers concurrently wait for it.
	for range 1000 {
		s.next = s.networkHead + 1
		s.checkDone()
		runtime.Gosched()
		s.networkHead = s.next
		s.checkDone()
		runtime.Gosched()
	}
	cancel()
	waiters.Wait()
}

func TestCoordinatorStateWaitCatchUpCycles(t *testing.T) {
	s := newCoordinatorState(DefaultParameters())
	waitCtx, waitCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer waitCancel()
	for range 2 {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		require.ErrorIs(t, s.waitCatchUp(ctx), context.Canceled)

		// A waiter started before completion must be notified by it.
		waited := make(chan error, 1)
		go func() { waited <- s.waitCatchUp(waitCtx) }()

		s.next = s.networkHead + 1
		s.checkDone()
		s.checkDone() // Rechecking completion must not close the channel twice.
		require.NoError(t, <-waited)
		require.NoError(t, s.waitCatchUp(waitCtx))
		require.NoError(t, s.waitCatchUp(ctx)) // Already complete takes precedence over cancellation.
		require.True(t, s.unsafeStats().CatchUpDone)

		s.networkHead = s.next
		s.checkDone()
		require.False(t, s.unsafeStats().CatchUpDone)
	}
}
