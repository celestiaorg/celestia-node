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

// SupplyIf supplies DI if a condition is met.
func SupplyIf(cond bool, val ...interface{}) fx.Option {
	if cond {
		return fx.Supply(val...)
	}
	return fx.Options()
}

// ProvideIf provides a given constructor if a condition is met.
func ProvideIf(cond bool, ctor ...interface{}) fx.Option {
	if cond {
		return fx.Provide(ctor...)
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

// ProvideAs creates an FX option that provides constructor 'cnstr' with the returned values types
// as 'cnstrs' It is as simple utility that hides away FX annotation details.
func ProvideAs(cnstr interface{}, cnstrs ...interface{}) fx.Option {
	return fx.Provide(fx.Annotate(cnstr, fx.As(cnstrs...)))
}

// ReplaceAs creates an FX option that substitutes types defined by constructors 'cnstrs' with the
// value 'val'. It is as simple utility that hides away FX annotation details.
func ReplaceAs(val interface{}, cnstrs ...interface{}) fx.Option {
	return fx.Replace(fx.Annotate(val, fx.As(cnstrs...)))
}
