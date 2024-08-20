package getters

import (
	"context"
	"errors"
	"time"

	logging "github.com/ipfs/go-log/v2"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/libs/utils"
	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds/byzantine"
	"github.com/celestiaorg/celestia-node/share/shwap"
)

var (
	tracer = otel.Tracer("share/getters")
	log    = logging.Logger("share/getters")
)

var _ shwap.Getter = (*CascadeGetter)(nil)

// CascadeGetter implements custom shwap.Getter that composes multiple Getter implementations in
// "cascading" order.
//
// See cascade func for details on cascading.
type CascadeGetter struct {
	getters []shwap.Getter
}

// NewCascadeGetter instantiates a new CascadeGetter from given shwap.Getters with given interval.
func NewCascadeGetter(getters []shwap.Getter) *CascadeGetter {
	return &CascadeGetter{
		getters: getters,
	}
}

// GetShare gets a share from any of registered shwap.Getters in cascading order.
func (cg *CascadeGetter) GetShare(
	ctx context.Context, header *header.ExtendedHeader, row, col int,
) (share.Share, error) {
	ctx, span := tracer.Start(ctx, "cascade/get-share", trace.WithAttributes(
		attribute.Int("row", row),
		attribute.Int("col", col),
	))
	defer span.End()

	upperBound := len(header.DAH.RowRoots)
	if row >= upperBound || col >= upperBound {
		err := shwap.ErrOutOfBounds
		span.RecordError(err)
		return nil, err
	}
	get := func(ctx context.Context, get shwap.Getter) (share.Share, error) {
		return get.GetShare(ctx, header, row, col)
	}

	return cascadeGetters(ctx, cg.getters, get)
}

// GetEDS gets a full EDS from any of registered shwap.Getters in cascading order.
func (cg *CascadeGetter) GetEDS(
	ctx context.Context, header *header.ExtendedHeader,
) (*rsmt2d.ExtendedDataSquare, error) {
	ctx, span := tracer.Start(ctx, "cascade/get-eds")
	defer span.End()

	get := func(ctx context.Context, get shwap.Getter) (*rsmt2d.ExtendedDataSquare, error) {
		return get.GetEDS(ctx, header)
	}

	return cascadeGetters(ctx, cg.getters, get)
}

// GetSharesByNamespace gets NamespacedShares from any of registered shwap.Getters in cascading
// order.
func (cg *CascadeGetter) GetSharesByNamespace(
	ctx context.Context,
	header *header.ExtendedHeader,
	namespace share.Namespace,
) (shwap.NamespaceData, error) {
	ctx, span := tracer.Start(ctx, "cascade/get-shares-by-namespace", trace.WithAttributes(
		attribute.String("namespace", namespace.String()),
	))
	defer span.End()

	get := func(ctx context.Context, get shwap.Getter) (shwap.NamespaceData, error) {
		return get.GetSharesByNamespace(ctx, header, namespace)
	}

	return cascadeGetters(ctx, cg.getters, get)
}

// cascade implements a cascading retry algorithm for getting a value from multiple sources.
// Cascading implies trying the sources one-by-one in the given order with the
// given interval until either:
//   - One of the sources returns the value
//   - All of the sources errors
//   - Context is canceled
//
// NOTE: New source attempts after interval do suspend running sources in progress.
func cascadeGetters[V any](
	ctx context.Context,
	getters []shwap.Getter,
	get func(context.Context, shwap.Getter) (V, error),
) (V, error) {
	var (
		zero V
		err  error
	)

	if len(getters) == 0 {
		return zero, errors.New("no getters provided")
	}

	ctx, span := tracer.Start(ctx, "cascade", trace.WithAttributes(
		attribute.Int("total-getters", len(getters)),
	))
	defer func() {
		if err != nil {
			utils.SetStatusAndEnd(span, errors.New("all getters failed"))
		}
	}()

	minTimeout := time.Duration(0)
	_, ok := ctx.Deadline()
	if !ok {
		// in this case minTimeout will be applied for all getters,so each of them
		// will have 1 minute timeout.
		minTimeout = time.Minute
	}

	for i, getter := range getters {
		log.Debugf("cascade: launching getter #%d", i)
		span.AddEvent("getter launched", trace.WithAttributes(attribute.Int("getter_idx", i)))

		// we split the timeout between left getters
		// once async cascadegetter is implemented, we can remove this
		getCtx, cancel := utils.CtxWithSplitTimeout(ctx, len(getters)-i, minTimeout)
		val, getErr := get(getCtx, getter)
		cancel()
		if getErr == nil {
			return val, nil
		}

		if errors.Is(getErr, shwap.ErrOperationNotSupported) {
			continue
		}

		span.RecordError(getErr, trace.WithAttributes(attribute.Int("getter_idx", i)))
		var byzantineErr *byzantine.ErrByzantine
		if errors.As(getErr, &byzantineErr) {
			// short circuit if byzantine error was detected (to be able to handle it correctly
			// and create the BEFP)
			return zero, byzantineErr
		}

		err = errors.Join(err, getErr)
		if ctx.Err() != nil {
			return zero, err
		}
	}
	return zero, err
}
