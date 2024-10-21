package store

import (
	"context"
	"errors"
	"fmt"

	libshare "github.com/celestiaorg/go-square/v2/share"
	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/libs/utils"
	"github.com/celestiaorg/celestia-node/share/eds"
	"github.com/celestiaorg/celestia-node/share/shwap"
)

var _ shwap.Getter = (*Getter)(nil)

type Getter struct {
	store *Store
}

func NewGetter(store *Store) *Getter {
	return &Getter{store: store}
}

func (g *Getter) GetShare(ctx context.Context, h *header.ExtendedHeader, row, col int) (libshare.Share, error) {
	acc, err := g.store.GetByHeight(ctx, h.Height())
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return libshare.Share{}, shwap.ErrNotFound
		}
		return libshare.Share{}, fmt.Errorf("get accessor from store:%w", err)
	}
	logger := log.With(
		"height", h.Height(),
		"row", row,
		"col", col,
	)
	defer utils.CloseAndLog(logger, "getter/sample", acc)

	sample, err := acc.Sample(ctx, row, col)
	if err != nil {
		return libshare.Share{}, fmt.Errorf("get sample from accessor:%w", err)
	}
	return sample.Share, nil
}

func (g *Getter) GetEDS(ctx context.Context, h *header.ExtendedHeader) (*rsmt2d.ExtendedDataSquare, error) {
	acc, err := g.store.GetByHeight(ctx, h.Height())
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, shwap.ErrNotFound
		}
		return nil, fmt.Errorf("get accessor from store:%w", err)
	}
	logger := log.With("height", h.Height())
	defer utils.CloseAndLog(logger, "getter/eds", acc)

	shares, err := acc.Shares(ctx)
	if err != nil {
		return nil, fmt.Errorf("get shares from accessor:%w", err)
	}
	rsmt2d, err := eds.Rsmt2DFromShares(shares, len(h.DAH.RowRoots)/2)
	if err != nil {
		return nil, fmt.Errorf("build eds from shares:%w", err)
	}
	return rsmt2d.ExtendedDataSquare, nil
}

func (g *Getter) GetSharesByNamespace(
	ctx context.Context,
	h *header.ExtendedHeader,
	ns libshare.Namespace,
) (shwap.NamespaceData, error) {
	acc, err := g.store.GetByHeight(ctx, h.Height())
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, shwap.ErrNotFound
		}
		return nil, fmt.Errorf("get accessor from store:%w", err)
	}
	logger := log.With(
		"height", h.Height(),
		"namespace", ns.String(),
	)
	defer utils.CloseAndLog(logger, "getter/nd", acc)

	nd, err := eds.NamespaceData(ctx, acc, ns)
	if err != nil {
		return nil, fmt.Errorf("get nd from accessor:%w", err)
	}
	return nd, nil
}
