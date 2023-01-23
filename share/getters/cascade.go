package getters

import (
	"context"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/multierr"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/nmt/namespace"
	"github.com/celestiaorg/rsmt2d"
)

var cascadeTracer = otel.Tracer("getters/cascade")

var _ share.Getter = (*CascadeGetter)(nil)

// CascadeGetter implements custom share.Getter that composes multiple Getter implementations in
// "cascading" order.
//
// See cascade func for details on cascading.
type CascadeGetter struct {
	interval time.Duration

	getters []share.Getter
}

// NewCascadeGetter instantiates a new CascadeGetter from given share.Getters with given interval.
func NewCascadeGetter(getters []share.Getter, interval time.Duration) *CascadeGetter {
	return &CascadeGetter{
		interval: interval,
		getters:  getters,
	}
}

// GetShare gets a share from any of registered share.Getters in cascading order.
func (cg *CascadeGetter) GetShare(ctx context.Context, root *share.Root, row, col int) (share.Share, error) {
	ctx, span := cascadeTracer.Start(ctx, "get-share", trace.WithAttributes(
		attribute.String("root", root.String()),
		attribute.Int("row", row),
		attribute.Int("col", col),
	))
	defer span.End()

	get := func(ctx context.Context, get share.Getter) (share.Share, error) {
		return get.GetShare(ctx, root, row, col)
	}

	return cascadeGetters(ctx, cg.getters, cg.interval, get)
}

// GetEDS gets a full EDS from any of registered share.Getters in cascading order.
func (cg *CascadeGetter) GetEDS(ctx context.Context, root *share.Root) (*rsmt2d.ExtendedDataSquare, error) {
	ctx, span := cascadeTracer.Start(ctx, "get-eds", trace.WithAttributes(
		attribute.String("root", root.String()),
	))
	defer span.End()

	get := func(ctx context.Context, get share.Getter) (*rsmt2d.ExtendedDataSquare, error) {
		return get.GetEDS(ctx, root)
	}

	return cascadeGetters(ctx, cg.getters, cg.interval, get)
}

// GetSharesByNamespace gets NamespacedShares from any of registered share.Getters in cascading
// order.
func (cg *CascadeGetter) GetSharesByNamespace(
	ctx context.Context,
	root *share.Root,
	id namespace.ID,
) (share.NamespacedShares, error) {
	ctx, span := cascadeTracer.Start(ctx, "get-shares-by-namespace", trace.WithAttributes(
		attribute.String("root", root.String()),
		attribute.String("nid", id.String()),
	))
	defer span.End()

	get := func(ctx context.Context, get share.Getter) (share.NamespacedShares, error) {
		return get.GetSharesByNamespace(ctx, root, id)
	}

	return cascadeGetters(ctx, cg.getters, cg.interval, get)
}

func cascadeGetters[V any](
	ctx context.Context,
	getters []share.Getter,
	interval time.Duration,
	get func(context.Context, share.Getter) (V, error),
) (V, error) {
	fns := make([]func(context.Context) (V, error), 0, len(getters))
	for _, getter := range getters {
		getter := getter // required for the same reason we do this when we launch goroutines in the loop
		fns = append(fns, func(ctx context.Context) (V, error) {
			return get(ctx, getter)
		})
	}

	return cascade[V](ctx, fns, interval)
}

// cascade implements a cascading retry algorithm for getting a value from multiple sources.
// Cascading implies trying the sources one-by-one in the given order with the
// given interval until either:
//   - One of the sources returns the value
//   - All of the sources errors
//   - Context is canceled
//
// NOTE: New source attempts after interval do suspend running sources in progress.
func cascade[V any](
	ctx context.Context,
	srcs []func(context.Context) (V, error),
	interval time.Duration,
) (V, error) {
	// short circuit when there is only one source
	if len(srcs) == 1 {
		return srcs[0](ctx)
	}
	// zero 'V'alue to return in error cases and errors themselves
	// NOTE: You cannot return nil for generics(type params) in Go
	var (
		zero V
		errs []error
	)

	ctx, span := cascadeTracer.Start(ctx, "cascade", trace.WithAttributes(
		attribute.String("interval", interval.String()),
		attribute.Int("total-sources", len(srcs)),
	))
	defer func() {
		defer span.End()
		if len(errs) == len(srcs) {
			// we do not set the actual errors to the description, as they were already recorded
			span.SetStatus(codes.Error, "all sources failed")
			return
		}

		span.SetStatus(codes.Ok, "")
	}()

	for i, src := range srcs {
		span.AddEvent("source launched", trace.WithAttributes(attribute.Int("source_id", i)))
		ctx, cancel := context.WithTimeout(ctx, interval)
		val, err := src(ctx)
		cancel()
		if err != nil {
			span.RecordError(err, trace.WithAttributes(attribute.Int("source_id", i)))
			errs = append(errs, err)
			continue
		}

		return val, nil
	}

	// TODO: migrate to errors.Join once Go1.20 is out!
	return zero, multierr.Combine(errs...)
}
