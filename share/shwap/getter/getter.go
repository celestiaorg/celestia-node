package shwap_getter

import (
	"context"
	"fmt"

	"github.com/ipfs/boxo/blockstore"
	"github.com/ipfs/boxo/exchange"
	block "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"

	"github.com/celestiaorg/celestia-app/pkg/wrapper"
	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/shwap"
)

type Getter struct {
	// TODO(@walldiss): why not blockservice?
	fetch  exchange.SessionExchange
	bstore blockstore.Blockstore
}

func NewGetter(fetch exchange.SessionExchange, bstore blockstore.Blockstore) *Getter {
	return &Getter{fetch: fetch, bstore: bstore}
}

func (g *Getter) GetShare(ctx context.Context, header *header.ExtendedHeader, row, col int) (share.Share, error) {
	shrIdx := row*len(header.DAH.RowRoots) + col
	shrs, err := g.GetShares(ctx, header, shrIdx)
	if err != nil {
		return nil, fmt.Errorf("getting shares: %w", err)
	}

	if len(shrs) != 1 {
		return nil, fmt.Errorf("expected 1 share, got %d", len(shrs))
	}

	return shrs[0], nil
}

// TODO: Make GetSamples so it provides proofs to users.
// GetShares fetches in the Block/EDS by their indexes.
// Automatically caches them on the Blockstore.
// Guarantee that the returned shares are in the same order as shrIdxs.
func (g *Getter) GetShares(ctx context.Context, hdr *header.ExtendedHeader, smplIdxs ...int) ([]share.Share, error) {
	if len(smplIdxs) == 0 {
		return nil, nil
	}

	if hdr.DAH.Equals(share.EmptyRoot()) {
		shares := make([]share.Share, len(smplIdxs))
		for _, idx := range smplIdxs {
			x, y := uint(smplIdxs[idx]/len(hdr.DAH.RowRoots)), uint(smplIdxs[idx]%len(hdr.DAH.RowRoots))
			shares[idx] = share.EmptyExtendedDataSquare().GetCell(x, y)
		}
		return shares, nil
	}

	cids := make([]cid.Cid, len(smplIdxs))
	for i, shrIdx := range smplIdxs {
		sid, err := shwap.NewSampleID(hdr.Height(), shrIdx, hdr.DAH)
		if err != nil {
			return nil, err
		}
		defer sid.Release()
		cids[i] = sid.Cid()
	}

	blks, err := g.getBlocks(ctx, cids)
	if err != nil {
		return nil, fmt.Errorf("getting blocks: %w", err)
	}

	// ensure we persist samples/blks and make them available for Bitswap
	err = g.bstore.PutMany(ctx, blks)
	if err != nil {
		return nil, fmt.Errorf("storing shares: %w", err)
	}
	// tell bitswap that we stored the blks and can serve them now
	err = g.fetch.NotifyNewBlocks(ctx, blks...)
	if err != nil {
		return nil, fmt.Errorf("notifying new shares: %w", err)
	}

	// ensure we return shares in the requested order
	shares := make(map[int]share.Share, len(blks))
	for _, blk := range blks {
		sample, err := shwap.SampleFromBlock(blk)
		if err != nil {
			return nil, fmt.Errorf("getting sample from block: %w", err)
		}
		shrIdx := int(sample.SampleID.RowIndex)*len(hdr.DAH.RowRoots) + int(sample.SampleID.ShareIndex)
		shares[shrIdx] = sample.SampleShare
	}

	ordered := make([]share.Share, len(shares))
	for i, shrIdx := range smplIdxs {
		sh, ok := shares[shrIdx]
		if !ok {
			return nil, fmt.Errorf("missing share for index %d", shrIdx)
		}
		ordered[i] = sh
	}

	return ordered, nil
}

// GetEDS
// TODO(@Wondertan): Consider requesting randomized rows instead of ODS only
func (g *Getter) GetEDS(ctx context.Context, hdr *header.ExtendedHeader) (*rsmt2d.ExtendedDataSquare, error) {
	if hdr.DAH.Equals(share.EmptyRoot()) {
		return share.EmptyExtendedDataSquare(), nil
	}

	sqrLn := len(hdr.DAH.RowRoots)
	cids := make([]cid.Cid, sqrLn/2)
	for i := 0; i < sqrLn/2; i++ {
		rid, err := shwap.NewRowID(hdr.Height(), uint16(i), hdr.DAH)
		if err != nil {
			return nil, err
		}
		defer rid.Release()
		cids[i] = rid.Cid()
	}

	blks, err := g.getBlocks(ctx, cids)
	if err != nil {
		return nil, fmt.Errorf("getting blocks: %w", err)

	}

	rows := make([]*shwap.Row, len(blks))
	for _, blk := range blks {
		row, err := shwap.RowFromBlock(blk)
		if err != nil {
			return nil, fmt.Errorf("getting row from block: %w", err)
		}
		rows[row.RowIndex] = row
	}

	shrs := make([]share.Share, 0, sqrLn*sqrLn)
	for _, row := range rows {
		shrs = append(shrs, row.RowShares...)
	}

	square, err := rsmt2d.ComputeExtendedDataSquare(
		shrs,
		share.DefaultRSMT2DCodec(),
		wrapper.NewConstructor(uint64(sqrLn/2)),
	)
	if err != nil {
		return nil, fmt.Errorf("computing EDS: %w", err)
	}

	// and try to repair
	err = square.Repair(hdr.DAH.RowRoots, hdr.DAH.ColumnRoots)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("repairing EDS: %w", err)
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

	from, to := share.RowRangeForNamespace(hdr.DAH, ns)
	if from == to {
		return share.NamespacedShares{}, nil
	}

	cids := make([]cid.Cid, 0, to-from)
	for rowIdx := from; rowIdx < to; rowIdx++ {
		did, err := shwap.NewDataID(hdr.Height(), uint16(rowIdx), ns, hdr.DAH)
		if err != nil {
			return nil, err
		}
		defer did.Release()
		cids = append(cids, did.Cid())
	}

	blks, err := g.getBlocks(ctx, cids)
	if err != nil {
		return nil, fmt.Errorf("getting blocks: %w", err)
	}

	nShrs := make([]share.NamespacedRow, len(blks))
	for _, blk := range blks {
		data, err := shwap.DataFromBlock(blk)
		if err != nil {
			return nil, fmt.Errorf("getting row from block: %w", err)
		}

		nShrs[int(data.RowIndex)-from] = share.NamespacedRow{
			Shares: data.DataShares,
			Proof:  &data.DataProof,
		}
	}

	return nShrs, nil
}

func (g *Getter) getBlocks(ctx context.Context, cids []cid.Cid) ([]block.Block, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	ses := g.fetch.NewSession(ctx)
	// must start getting only after verifiers are registered
	blkCh, err := ses.GetBlocks(ctx, cids)
	if err != nil {
		return nil, fmt.Errorf("fetching blocks: %w", err)
	}
	// GetBlocks handles ctx and closes blkCh, so we don't have to
	blks := make([]block.Block, 0, len(cids))
	for blk := range blkCh {
		blks = append(blks, blk)
	}
	// only persist when all samples received
	if len(blks) != len(cids) {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("not all blocks were found")
	}

	return blks, nil
}
