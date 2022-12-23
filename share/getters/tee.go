package getters

import (
	"context"
	"errors"
	"fmt"

	"github.com/filecoin-project/dagstore"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds"

	"github.com/celestiaorg/nmt/namespace"
	"github.com/celestiaorg/rsmt2d"
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

func (tg *TeeGetter) GetShare(ctx context.Context, root *share.Root, row, col int) (share.Share, error) {
	return tg.getter.GetShare(ctx, root, row, col)
}

func (tg *TeeGetter) GetEDS(ctx context.Context, root *share.Root) (*rsmt2d.ExtendedDataSquare, error) {
	eds, err := tg.getter.GetEDS(ctx, root)
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
	id namespace.ID,
) (share.NamespaceShares, error) {
	return tg.getter.GetSharesByNamespace(ctx, root, id)
}
