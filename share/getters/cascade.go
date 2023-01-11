package getters

import (
	"context"
	"time"

	"go.uber.org/multierr"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/nmt/namespace"
	"github.com/celestiaorg/rsmt2d"
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
	get := func(ctx context.Context, get share.Getter) (share.Share, error) {
		return get.GetShare(ctx, root, row, col)
	}
	return cascadeGetters(ctx, cg.getters, cg.interval, get)
}

// GetEDS gets a full EDS from any of registered share.Getters in cascading order.
func (cg *CascadeGetter) GetEDS(ctx context.Context, root *share.Root) (*rsmt2d.ExtendedDataSquare, error) {
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
// NOTE: New source attempts after interval do not suspend running sources in progress.
// TODO(@Wondertan): Move to utils
func cascade[V any](ctx context.Context, srcs []func(context.Context) (V, error), interval time.Duration) (V, error) {
	// short circuit when there is only func to cascade
	if len(srcs) == 1 {
		return srcs[0](ctx)
	}
	// once we got value from one of the fns
	// this cancels all others on return
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	// start from zero so the first iteration is instantaneous
	// later on in the loop below we reset to real interval
	t := time.NewTimer(0)
	defer t.Stop()
	// zero 'V'alue to return in error cases and errors themselves
	// NOTE: You cannot return nil for generics(type params) in Go
	var (
		zero V
		errs []error
	)
	// results channel with anon type
	// NOTE: Unfortunately, you cannot define private types in generic funcs in Go
	results := make(chan struct {
		val V
		err error
	})

	for _, src := range srcs {
		select {
		case res := <-results:
			if res.err == nil {
				return res.val, nil
			}
			errs = append(errs, res.err)
		case <-ctx.Done():
			return zero, ctx.Err()
		case <-t.C:
		}

		t.Reset(interval)
		go func(src func(context.Context) (V, error)) {
			val, err := src(ctx)
			select {
			case results <- struct {
				val V
				err error
			}{val: val, err: err}:
			case <-ctx.Done():
			}
		}(src)
	}

	// we know how many sources were executed in total
	// and how many were processed already, so expect only the diff
	for i := len(errs); i < len(srcs); i++ {
		select {
		case res := <-results:
			if res.err == nil {
				return res.val, nil
			}
			errs = append(errs, res.err)
		case <-ctx.Done():
			return zero, ctx.Err()
		}
	}

	// TODO: migrate to errors.Join once Go1.20 is out!
	return zero, multierr.Combine(errs...)
}
