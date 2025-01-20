package utils

import (
	"context"
	"time"
)

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
