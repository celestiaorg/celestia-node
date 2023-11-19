package utils

import "context"

// ResetContextOnError returns the context if there is no error, otherwise returns it after resetting it to background.
func ResetContextOnError(ctx context.Context) context.Context {
	if ctx.Err() != nil {
		ctx = context.Background()
	}

	return ctx
}
