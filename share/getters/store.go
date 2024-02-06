package getters

import (
	"context"
	"errors"
	"fmt"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/libs/utils"
	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/store"
)

var _ share.Getter = (*StoreGetter)(nil)

// StoreGetter is a share.Getter that retrieves shares from an eds.Store. No results are saved to
// the eds.Store after retrieval.
type StoreGetter struct {
	store *store.Store
}

// NewStoreGetter creates a new share.Getter that retrieves shares from an eds.Store.
func NewStoreGetter(store *store.Store) *StoreGetter {
	return &StoreGetter{
		store: store,
	}
}

// GetShare gets a single share at the given EDS coordinates from the eds.Store through the
// corresponding CAR-level blockstore.
func (sg *StoreGetter) GetShare(ctx context.Context, header *header.ExtendedHeader, row, col int) (share.Share, error) {
	dah := header.DAH
	var err error
	ctx, span := tracer.Start(ctx, "store/get-share", trace.WithAttributes(
		attribute.Int("row", row),
		attribute.Int("col", col),
	))
	defer func() {
		utils.SetStatusAndEnd(span, err)
	}()

	upperBound := len(dah.RowRoots)
	if row >= upperBound || col >= upperBound {
		err := share.ErrOutOfBounds
		span.RecordError(err)
		return nil, err
	}

	file, err := sg.store.GetByHash(ctx, dah.Hash())
	if errors.Is(err, store.ErrNotFound) {
		// convert error to satisfy getter interface contract
		err = share.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("getter/store: failed to retrieve file: %w", err)
	}
	defer utils.CloseAndLog(log, "file", file)

	sh, err := file.Share(ctx, col, row)
	if errors.Is(err, store.ErrNotFound) {
		// convert error to satisfy getter interface contract
		err = share.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("getter/store: failed to retrieve share: %w", err)
	}

	return sh.Share, nil
}

// GetEDS gets the EDS identified by the given root from the EDS store.
func (sg *StoreGetter) GetEDS(
	ctx context.Context, header *header.ExtendedHeader,
) (data *rsmt2d.ExtendedDataSquare, err error) {
	ctx, span := tracer.Start(ctx, "store/get-eds")
	defer func() {
		utils.SetStatusAndEnd(span, err)
	}()

	file, err := sg.store.GetByHash(ctx, header.DAH.Hash())
	if errors.Is(err, store.ErrNotFound) {
		// convert error to satisfy getter interface contract
		err = share.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("getter/store: failed to retrieve file: %w", err)
	}
	defer utils.CloseAndLog(log, "file", file)

	eds, err := file.EDS(ctx)
	if err != nil {
		return nil, fmt.Errorf("getter/store: failed to retrieve eds: %w", err)
	}
	return eds, nil
}

// GetSharesByNamespace gets all EDS shares in the given namespace from the EDS store through the
// corresponding CAR-level blockstore.
func (sg *StoreGetter) GetSharesByNamespace(
	ctx context.Context,
	header *header.ExtendedHeader,
	namespace share.Namespace,
) (shares share.NamespacedShares, err error) {
	ctx, span := tracer.Start(ctx, "store/get-shares-by-namespace", trace.WithAttributes(
		attribute.String("namespace", namespace.String()),
	))
	defer func() {
		utils.SetStatusAndEnd(span, err)
	}()

	file, err := sg.store.GetByHeight(ctx, header.Height())
	if errors.Is(err, store.ErrNotFound) {
		// convert error to satisfy getter interface contract
		err = share.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("getter/store: failed to retrieve file: %w", err)
	}
	defer utils.CloseAndLog(log, "file", file)

	// get all shares in the namespace
	from, to := share.RowRangeForNamespace(header.DAH, namespace)

	shares = make(share.NamespacedShares, 0, to-from+1)
	for row := from; row < to; row++ {
		data, err := file.Data(ctx, namespace, row)
		if err != nil {
			return nil, fmt.Errorf("getter/store: failed to retrieve namespcaed data: %w", err)
		}
		shares = append(shares, data)
	}

	return shares, nil
}
