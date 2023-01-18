package getters

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/celestiaorg/celestia-node/libs/utils"
	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds"
	"github.com/celestiaorg/celestia-node/share/ipld"

	"github.com/celestiaorg/nmt/namespace"
	"github.com/celestiaorg/rsmt2d"
)

var _ share.Getter = (*StoreGetter)(nil)

// StoreGetter is a share.Getter that retrieves shares from an eds.Store. No results are saved to
// the eds.Store after retrieval.
type StoreGetter struct {
	store *eds.Store
}

// NewStoreGetter creates a new share.Getter that retrieves shares from an eds.Store.
func NewStoreGetter(store *eds.Store) *StoreGetter {
	return &StoreGetter{
		store: store,
	}
}

// GetShare gets a single share at the given EDS coordinates from the eds.Store through the
// corresponding CAR-level blockstore.
func (sg *StoreGetter) GetShare(ctx context.Context, dah *share.Root, row, col int) (share.Share, error) {
	var err error
	ctx, span := tracer.Start(ctx, "store/get-share", trace.WithAttributes(
		attribute.String("root", dah.String()),
		attribute.Int("row", row),
		attribute.Int("col", col),
	))
	defer func() {
		utils.SetStatusAndEnd(span, err)
	}()

	root, leaf := ipld.Translate(dah, row, col)
	bs, err := sg.store.CARBlockstore(ctx, dah.Hash())
	if err != nil {
		return nil, fmt.Errorf("getter/store: failed to retrieve blockstore: %w", err)
	}

	// wrap the read-only CAR blockstore in a getter
	blockGetter := eds.NewBlockGetter(bs)
	share, err := share.GetShare(ctx, blockGetter, root, leaf, len(dah.RowsRoots))
	if err != nil {
		return nil, fmt.Errorf("getter/store: failed to retrieve share: %w", err)
	}

	return share, nil
}

// GetEDS gets the EDS identified by the given root from the EDS store.
func (sg *StoreGetter) GetEDS(ctx context.Context, root *share.Root) (eds *rsmt2d.ExtendedDataSquare, err error) {
	ctx, span := tracer.Start(ctx, "store/get-eds", trace.WithAttributes(
		attribute.String("root", root.String()),
	))
	defer func() {
		utils.SetStatusAndEnd(span, err)
	}()

	eds, err = sg.store.Get(ctx, root.Hash())
	if err != nil {
		return nil, fmt.Errorf("getter/store: failed to retrieve eds: %w", err)
	}
	return eds, nil
}

// GetSharesByNamespace gets all EDS shares in the given namespace from the EDS store through the
// corresponding CAR-level blockstore.
func (sg *StoreGetter) GetSharesByNamespace(
	ctx context.Context,
	root *share.Root,
	nID namespace.ID,
) (shares share.NamespacedShares, err error) {
	ctx, span := tracer.Start(ctx, "store/get-shares-by-namespace", trace.WithAttributes(
		attribute.String("root", root.String()),
		attribute.String("nID", nID.String()),
	))
	defer func() {
		utils.SetStatusAndEnd(span, err)
	}()

	err = verifyNIDSize(nID)
	if err != nil {
		return nil, fmt.Errorf("getter/store: invalid namespace ID: %w", err)
	}

	bs, err := sg.store.CARBlockstore(ctx, root.Hash())
	if err != nil {
		return nil, fmt.Errorf("getter/store: failed to retrieve blockstore: %w", err)
	}

	// wrap the read-only CAR blockstore in a getter
	blockGetter := eds.NewBlockGetter(bs)
	shares, err = collectSharesByNamespace(ctx, blockGetter, root, nID)
	if err != nil {
		return nil, fmt.Errorf("getter/store: failed to retrieve shares by namespace: %w", err)
	}
	return shares, nil
}
