package utils

import (
	"context"
)

// ResetContextOnError returns a fresh context if the given context has an error.
func ResetContextOnError(ctx context.Context) context.Context {
	if ctx.Err() != nil {
		ctx = context.Background()
	}

	return ctx
}
