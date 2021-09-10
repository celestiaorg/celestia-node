package fxutil

import (
	"context"

	"go.uber.org/fx"
)

// WithLifecycle wraps a context to be canceled when the lifecycle stops.
func WithLifecycle(ctx context.Context, lc fx.Lifecycle) context.Context {
	ctx, cancel := context.WithCancel(ctx)
	lc.Append(fx.Hook{
		OnStop: func(_ context.Context) error {
			cancel()
			return nil
		},
	})
	return ctx
}
