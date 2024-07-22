package bitswap

import (
	"context"
	"fmt"

	"github.com/ipfs/boxo/blockstore"
	"github.com/ipfs/boxo/exchange"

	"github.com/celestiaorg/celestia-app/pkg/wrapper"
	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/share"
)

// Getter implements share.Getter.
type Getter struct {
	exchange exchange.SessionExchange
	bstore   blockstore.Blockstore
	session  exchange.Fetcher
	cancel   context.CancelFunc
}

// NewGetter constructs a new Getter.
func NewGetter(exchange exchange.SessionExchange, bstore blockstore.Blockstore) *Getter {
	return &Getter{exchange: exchange, bstore: bstore}
}

// Start kicks off internal fetching session.
func (g *Getter) Start() {
	ctx, cancel := context.WithCancel(context.Background())
	g.session = g.exchange.NewSession(ctx)
	g.cancel = cancel
}

// Stop shuts down Getter's internal fetching session.
func (g *Getter) Stop() {
	g.cancel()
}

// TODO(@Wondertan): Rework API to get coordinates as a single param to make it ergonomic.
func (g *Getter) GetShares(
	ctx context.Context,
	hdr *header.ExtendedHeader,
	rowIdxs, colIdxs []int,
) ([]share.Share, error) {
	if len(rowIdxs) != len(colIdxs) {
		return nil, fmt.Errorf("row indecies and col indecies must be same length")
	}

	if len(rowIdxs) == 0 {
		return nil, fmt.Errorf("empty coordinates")
	}

	blks := make([]Block, 0, len(rowIdxs))
	for i, rowIdx := range rowIdxs {
		sid, err := NewEmptySampleBlock(hdr.Height(), rowIdx, colIdxs[i], len(hdr.DAH.RowRoots))
		if err != nil {
			return nil, err
		}

		blks[i] = sid
	}

	err := Fetch(ctx, g.exchange, hdr.DAH, blks, WithStore(g.bstore))
	if err != nil {
		return nil, err
	}

	shares := make([]share.Share, len(blks))
	for i, blk := range blks {
		shares[i] = blk.(*SampleBlock).Container.Share
	}

	return shares, nil
}

func (g *Getter) GetShare(
	ctx context.Context,
	hdr *header.ExtendedHeader,
	row, col int,
) (share.Share, error) {
	shrs, err := g.GetShares(ctx, hdr, []int{row}, []int{col})
	if err != nil {
		return nil, err
	}

	if len(shrs) != 1 {
		return nil, fmt.Errorf("expected 1 share row, got %d", len(shrs))
	}

	return shrs[0], nil
}

func (g *Getter) GetEDS(
	ctx context.Context,
	hdr *header.ExtendedHeader,
) (*rsmt2d.ExtendedDataSquare, error) {
	sqrLn := len(hdr.DAH.RowRoots)
	blks := make([]Block, sqrLn/2)
	for i := 0; i < sqrLn/2; i++ {
		blk, err := NewEmptyRowBlock(hdr.Height(), i, sqrLn)
		if err != nil {
			return nil, err
		}

		blks[i] = blk
	}

	err := Fetch(ctx, g.exchange, hdr.DAH, blks, WithFetcher(g.session))
	if err != nil {
		return nil, err
	}

	shrs := make([]share.Share, 0, sqrLn*sqrLn)
	for _, row := range blks {
		rowShrs, err := row.(*RowBlock).Container.Shares()
		if err != nil {
			return nil, fmt.Errorf("decoding Shares out of Row: %w", err)
		}
		shrs = append(shrs, rowShrs...)
	}

	square, err := rsmt2d.ComputeExtendedDataSquare(
		shrs,
		share.DefaultRSMT2DCodec(),
		wrapper.NewConstructor(uint64(sqrLn/2)),
	)
	if err != nil {
		return nil, fmt.Errorf("computing EDS: %w", err)
	}

	return square, nil
}

func (g *Getter) GetSharesByNamespace(
	ctx context.Context,
	hdr *header.ExtendedHeader,
	ns share.Namespace,
) (share.NamespacedShares, error) {
	if err := ns.ValidateForData(); err != nil {
		return nil, err
	}

	rowIdxs := share.RowsWithNamespace(hdr.DAH, ns)
	blks := make([]Block, len(rowIdxs))
	for i, rowNdIdx := range rowIdxs {
		rndblk, err := NewEmptyRowNamespaceDataBlock(hdr.Height(), rowNdIdx, ns, len(hdr.DAH.RowRoots))
		if err != nil {
			return nil, err
		}
		blks[i] = rndblk
	}

	err := Fetch(ctx, g.exchange, hdr.DAH, blks, WithFetcher(g.session))
	if err != nil {
		return nil, err
	}

	// TODO(@Wondertan): this must use shwap types eventually
	nsShrs := make(share.NamespacedShares, len(blks))
	for i, blk := range blks {
		rnd := blk.(*RowNamespaceDataBlock).Container
		nsShrs[i] = share.NamespacedRow{
			Shares: rnd.Shares,
			Proof:  rnd.Proof,
		}
	}

	return nsShrs, nil
}
