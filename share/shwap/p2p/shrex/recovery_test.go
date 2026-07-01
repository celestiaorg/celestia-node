package shrex

import (
	"sync/atomic"
	"testing"

	"github.com/libp2p/go-libp2p/core/network"
	"github.com/stretchr/testify/require"
)

// resetSpyStream wraps a network.Stream and records whether Reset was called.
// The embedded nil interface is never dereferenced because the handlers under
// test only ever call Reset on it.
type resetSpyStream struct {
	network.Stream
	reset atomic.Bool
}

func (s *resetSpyStream) Reset() error {
	s.reset.Store(true)
	return nil
}

// TestRecoveryMiddleware_ResetsStreamOnPanic verifies that when the wrapped
// handler panics, RecoveryMiddleware recovers and resets the stream so the
// half-handled connection is torn down instead of lingering open and leaking a
// stream slot until the resource-manager timeout.
func TestRecoveryMiddleware_ResetsStreamOnPanic(t *testing.T) {
	spy := &resetSpyStream{}

	handler := RecoveryMiddleware(func(network.Stream) {
		panic("boom")
	})

	require.NotPanics(t, func() { handler(spy) }, "middleware must recover the panic")
	require.True(t, spy.reset.Load(), "stream must be reset after a handler panic")
}

// TestRecoveryMiddleware_NoResetWithoutPanic verifies the middleware does not
// reset the stream on the normal (non-panicking) path.
func TestRecoveryMiddleware_NoResetWithoutPanic(t *testing.T) {
	spy := &resetSpyStream{}

	handler := RecoveryMiddleware(func(network.Stream) {
		// normal completion, no panic
	})

	handler(spy)
	require.False(t, spy.reset.Load(), "stream must not be reset when the handler returns normally")
}
