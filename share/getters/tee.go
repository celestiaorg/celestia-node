package getters

import (
	"context"
	"errors"
	"fmt"

	"github.com/filecoin-project/dagstore"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/libs/utils"
	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds"
)

var _ share.Getter = (*TeeGetter)(nil)

// TeeGetter is a share.Getter that wraps a getter and stores the results of GetEDS into an
// eds.Store.
type TeeGetter struct {
	getter share.Getter
	store  *eds.Store
}

// NewTeeGetter creates a new TeeGetter.
func NewTeeGetter(getter share.Getter, store *eds.Store) *TeeGetter {
	return &TeeGetter{
		getter: getter,
		store:  store,
	}
}

func (tg *TeeGetter) GetShare(ctx context.Context, root *share.Root, row, col int) (share share.Share, err error) {
	ctx, span := tracer.Start(ctx, "tee/get-share", trace.WithAttributes(
		attribute.String("root", root.String()),
		attribute.Int("row", row),
		attribute.Int("col", col),
	))
	defer func() {
		utils.SetStatusAndEnd(span, err)
	}()

	return tg.getter.GetShare(ctx, root, row, col)
}

func (tg *TeeGetter) GetEDS(ctx context.Context, root *share.Root) (eds *rsmt2d.ExtendedDataSquare, err error) {
	ctx, span := tracer.Start(ctx, "tee/get-eds", trace.WithAttributes(
		attribute.String("root", root.String()),
	))
	defer func() {
		utils.SetStatusAndEnd(span, err)
	}()

	eds, err = tg.getter.GetEDS(ctx, root)
	if err != nil {
		return nil, err
	}

	err = tg.store.Put(ctx, root.Hash(), eds)
	if err != nil && !errors.Is(err, dagstore.ErrShardExists) {
		return nil, fmt.Errorf("getter/tee: failed to store eds: %w", err)
	}

	return eds, nil
}

func (tg *TeeGetter) GetSharesByNamespace(
	ctx context.Context,
	root *share.Root,
	namespace share.Namespace,
) (shares share.NamespacedShares, err error) {
	ctx, span := tracer.Start(ctx, "tee/get-shares-by-namespace", trace.WithAttributes(
		attribute.String("root", root.String()),
		attribute.String("namespace", namespace.String()),
	))
	defer func() {
		utils.SetStatusAndEnd(span, err)
	}()

	return tg.getter.GetSharesByNamespace(ctx, root, namespace)
}
