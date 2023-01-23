package getters

import (
	"context"
	"time"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/nmt/namespace"
	"github.com/celestiaorg/rsmt2d"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/multierr"
)

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
	ctx, span := tracer.Start(ctx, "cascade/get-share", trace.WithAttributes(
		attribute.String("root", root.String()),
		attribute.Int("row", row),
		attribute.Int("col", col),
	))
	defer span.End()

	get := func(ctx context.Context, get share.Getter) (share.Share, error) {
		return get.GetShare(ctx, root, row, col)
	}

	return cascadeGetters(ctx, cg.getters, get, cg.interval)
}

// GetEDS gets a full EDS from any of registered share.Getters in cascading order.
func (cg *CascadeGetter) GetEDS(ctx context.Context, root *share.Root) (*rsmt2d.ExtendedDataSquare, error) {
	ctx, span := tracer.Start(ctx, "cascade/get-eds", trace.WithAttributes(
		attribute.String("root", root.String()),
	))
	defer span.End()

	get := func(ctx context.Context, get share.Getter) (*rsmt2d.ExtendedDataSquare, error) {
		return get.GetEDS(ctx, root)
	}

	return cascadeGetters(ctx, cg.getters, get, cg.interval)
}

// GetSharesByNamespace gets NamespacedShares from any of registered share.Getters in cascading
// order.
func (cg *CascadeGetter) GetSharesByNamespace(
	ctx context.Context,
	root *share.Root,
	id namespace.ID,
) (share.NamespacedShares, error) {
	ctx, span := tracer.Start(ctx, "cascade/gget-shares-by-namespace", trace.WithAttributes(
		attribute.String("root", root.String()),
		attribute.String("nid", id.String()),
	))
	defer span.End()

	get := func(ctx context.Context, get share.Getter) (share.NamespacedShares, error) {
		return get.GetSharesByNamespace(ctx, root, id)
	}

	return cascadeGetters(ctx, cg.getters, get, cg.interval)
}

// getVal defines a type constraint for the types allowed for cascading
type getVal interface {
	share.Share | share.NamespacedShares | *rsmt2d.ExtendedDataSquare
}

// cascade implements a cascading retry algorithm for getting a value from multiple sources.
// Cascading implies trying the sources one-by-one in the given order with the
// given interval until either:
//   - One of the sources returns the value
//   - All of the sources errors
//   - Context is canceled
//
// NOTE: New source attempts after interval do suspend running sources in progress.
func cascadeGetters[V getVal](
	ctx context.Context,
	getters []share.Getter,
	get func(context.Context, share.Getter) (V, error),
	interval time.Duration,
) (zero V,  err error) {
	ctx, span := tracer.Start(ctx, "cascade", trace.WithAttributes(
		attribute.String("interval", interval.String()),
		attribute.Int("total-getters", len(getters)),
	))
	defer func() {
		defer span.End()
		if err != nil {
			// we do not set the actual errors to the description, as they were already recorded
			span.SetStatus(codes.Error, "all getters failed")
			return
		}
		span.SetStatus(codes.Ok, "")
	}()

	for i, getter := range getters {
		span.AddEvent("getter launched", trace.WithAttributes(attribute.Int("getter_id", i)))
		ctx, cancel := context.WithTimeout(ctx, interval)
		val, err := get(ctx, getter)
		cancel()
		if err == nil {
			return val, nil
		}

		// TODO(@Wondertan): migrate to errors.Join once Go1.20 is out!
		err = multierr.Append(err, err)
		span.RecordError(err, trace.WithAttributes(attribute.Int("getter_id", i)))
	}
	return
}
