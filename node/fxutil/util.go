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

// ProvideIf provides a given constructor if a condition is met.
func ProvideIf(cond bool, ctor interface{}) fx.Option {
	if cond {
		return fx.Provide(ctor)
	}

	return fx.Options()
}

// InvokeIf invokes a given function if a condition is met.
func InvokeIf(cond bool, function interface{}) fx.Option {
	if cond {
		return fx.Invoke(function)
	}
	return fx.Options()
}
