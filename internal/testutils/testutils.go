package testutils

import (
	"context"
	"testing"
	"time"
)

// Context for testing purpose. Will be automatically canceled on test finish.
func Context(tb testing.TB, timeout time.Duration) context.Context {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	tb.Cleanup(cancel)
	return ctx
}
