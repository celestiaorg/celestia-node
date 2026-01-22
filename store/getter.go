package store

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"

	libshare "github.com/celestiaorg/go-square/v3/share"
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

func (g *Getter) GetSamples(ctx context.Context, hdr *header.ExtendedHeader,
	indices []shwap.SampleCoords,
) ([]shwap.Sample, error) {
	acc, err := g.store.GetByHeight(ctx, hdr.Height())
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, shwap.ErrNotFound
		}
		return nil, fmt.Errorf("get accessor from store:%w", err)
	}
	defer utils.CloseAndLog(log.With("height", hdr.Height()), "getter/sample", acc)

	smpls := make([]shwap.Sample, len(indices))
	for i, idx := range indices {
		smpl, err := acc.Sample(ctx, idx)
		if err != nil {
			return nil, fmt.Errorf("get sample from accessor:%w", err)
		}

		smpls[i] = smpl
	}

	return smpls, nil
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

func (g *Getter) GetRow(ctx context.Context, h *header.ExtendedHeader, rowIdx int) (shwap.Row, error) {
	acc, err := g.store.GetByHeight(ctx, h.Height())
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return shwap.Row{}, shwap.ErrNotFound
		}
		return shwap.Row{}, fmt.Errorf("getting accessor from store: %w", err)
	}
	logger := log.With("height", h.Height())
	defer utils.CloseAndLog(logger, "getter/row", acc)
	axisHalf, err := acc.AxisHalf(ctx, rsmt2d.Row, rowIdx)
	if err != nil {
		return shwap.Row{}, fmt.Errorf("getting axis half from accessor: %w", err)
	}
	return axisHalf.ToRow(), nil
}

func (g *Getter) GetNamespaceData(
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

func (g *Getter) GetRangeNamespaceData(
	ctx context.Context,
	h *header.ExtendedHeader,
	from, to int,
) (shwap.RangeNamespaceData, error) {
	acc, err := g.store.GetByHeight(ctx, h.Height())
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return shwap.RangeNamespaceData{}, shwap.ErrNotFound
		}
		return shwap.RangeNamespaceData{}, fmt.Errorf("getting accessor from store:%w", err)
	}

	logger := log.With(
		"height", h.Height(),
		"from", from,
		"to", to,
	)
	defer utils.CloseAndLog(logger, "getter/rng", acc)
	rngData, err := acc.RangeNamespaceData(ctx, from, to)
	if err != nil {
		return shwap.RangeNamespaceData{}, fmt.Errorf("getting range from accessor:%w", err)
	}
	return rngData, nil
}

func (g *Getter) GetBlob(
	ctx context.Context,
	h *header.ExtendedHeader,
	namespace libshare.Namespace,
	commitment []byte,
) (*shwap.Blob, error) {
	acc, err := g.store.GetByHeight(ctx, h.Height())
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, shwap.ErrNotFound
		}
		return nil, fmt.Errorf("getting accessor from store:%w", err)
	}
	logger := log.With(
		"height", h.Height(),
		"namespace", namespace.String(),
		"commitment", hex.EncodeToString(commitment),
	)
	defer utils.CloseAndLog(logger, "getter/blob", acc)
	return acc.Blob(ctx, namespace, commitment)
}

func (g *Getter) GetBlobs(
	ctx context.Context,
	h *header.ExtendedHeader,
	namespace libshare.Namespace,
) ([]*shwap.Blob, error) {
	acc, err := g.store.GetByHeight(ctx, h.Height())
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, shwap.ErrNotFound
		}
		return nil, fmt.Errorf("getting accessor from store:%w", err)
	}
	logger := log.With(
		"height", h.Height(),
		"namespace", namespace.String(),
	)
	defer utils.CloseAndLog(logger, "getter/blobs", acc)
	return acc.Blobs(ctx, namespace)
}
