package utils

import (
	"context"
	"time"
)

// StartupErrorBuffer is how long before a startup deadline a startup hook should
// abort so its descriptive error is returned before fx masks it with a bare
// context.DeadlineExceeded. fx's withTimeout prefers ctx.Err() whenever the startup
// context is already expired at the moment the hook returns, so a hook that only
// fails exactly at the deadline loses its error; failing slightly earlier keeps it.
const StartupErrorBuffer = time.Second

// CtxWithStartupBuffer derives a child context whose deadline is StartupErrorBuffer
// before the parent's deadline. If the parent has no deadline, or the buffered
// deadline is already in the past, the parent is returned unchanged.
func CtxWithStartupBuffer(ctx context.Context) (context.Context, context.CancelFunc) {
	deadline, ok := ctx.Deadline()
	if !ok {
		return context.WithCancel(ctx)
	}
	buffered := deadline.Add(-StartupErrorBuffer)
	if !buffered.After(time.Now()) {
		return context.WithCancel(ctx)
	}
	return context.WithDeadline(ctx, buffered)
}

// ResetContextOnError returns a fresh context if the given context has an error.
func ResetContextOnError(ctx context.Context) context.Context {
	if ctx.Err() != nil {
		ctx = context.Background()
	}

	return ctx
}

// CtxWithSplitTimeout will split timeout stored in context by splitFactor and return the result if
// it is greater than minTimeout. minTimeout == 0 will be ignored, splitFactor <= 0 will be ignored
func CtxWithSplitTimeout(
	ctx context.Context,
	splitFactor int,
	minTimeout time.Duration,
) (context.Context, context.CancelFunc) {
	deadline, ok := ctx.Deadline()
	if !ok || splitFactor <= 0 {
		if minTimeout == 0 {
			return context.WithCancel(ctx)
		}
		return context.WithTimeout(ctx, minTimeout)
	}

	timeout := time.Until(deadline)
	if timeout < minTimeout {
		return context.WithCancel(ctx)
	}

	splitTimeout := timeout / time.Duration(splitFactor)
	if splitTimeout < minTimeout {
		return context.WithTimeout(ctx, minTimeout)
	}
	return context.WithTimeout(ctx, splitTimeout)
}
