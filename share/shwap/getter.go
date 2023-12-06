package shwap

import (
	"context"
	"fmt"
	"slices"

	"github.com/ipfs/boxo/blockservice"
	"github.com/ipfs/go-cid"

	"github.com/celestiaorg/celestia-app/pkg/wrapper"
	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/share"
)

type Getter struct {
	bget blockservice.BlockGetter
}

func NewGetter(bget blockservice.BlockGetter) *Getter {
	return &Getter{bget: bget}
}

// GetShare
// TODO: Deprecate this method
func (g *Getter) GetShare(ctx context.Context, hdr *header.ExtendedHeader, row, col int) (share.Share, error) {
	shrIdx := row*len(hdr.DAH.RowRoots) + col
	cid := MustSampleCID(shrIdx, hdr.DAH, hdr.Height())
	blk, err := g.bget.GetBlock(ctx, cid)
	if err != nil {
		return nil, fmt.Errorf("getting block from blockservice: %w", err)
	}

	smpl, err := SampleFromBlock(blk)
	if err != nil {
		return nil, fmt.Errorf("converting block to Sample: %w", err)
	}

	return smpl.SampleShare, nil
}

// GetShares
// Guarantee that the returned shares are in the same order as shrIdxs.
func (g *Getter) GetShares(ctx context.Context, hdr *header.ExtendedHeader, shrIdxs ...int) ([]share.Share, error) {
	maxIdx := len(hdr.DAH.RowRoots) * len(hdr.DAH.ColumnRoots)
	cids := make([]cid.Cid, len(shrIdxs))
	for i, shrIdx := range shrIdxs {
		if shrIdx < 0 || shrIdx >= maxIdx {
			return nil, fmt.Errorf("share index %d is out of bounds", shrIdx)
		}
		cids[i] = MustSampleCID(shrIdx, hdr.DAH, hdr.Height())
	}

	smpls := make(map[int]*Sample, len(shrIdxs))
	blkCh := g.bget.GetBlocks(ctx, cids)
	for blk := range blkCh { // NOTE: GetBlocks handles ctx, so we don't have to
		smpl, err := SampleFromBlock(blk)
		if err != nil {
			// NOTE: Should never error in fact, as Hasher already validated the block
			return nil, fmt.Errorf("converting block to Sample: %w", err)
		}

		shrIdx := int(smpl.SampleID.AxisIndex)*len(hdr.DAH.RowRoots) + int(smpl.SampleID.ShareIndex)
		smpls[shrIdx] = smpl
	}

	if len(smpls) != len(shrIdxs) {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("not all shares were found")
	}

	shrs := make([]share.Share, len(shrIdxs))
	for i, shrIdx := range shrIdxs {
		shrs[i] = smpls[shrIdx].SampleShare
	}

	return shrs, nil
}

// GetEDS
// TODO(@Wondertan): Consider requesting randomized rows and cols instead of ODS only
func (g *Getter) GetEDS(ctx context.Context, hdr *header.ExtendedHeader) (*rsmt2d.ExtendedDataSquare, error) {
	sqrLn := len(hdr.DAH.RowRoots)
	cids := make([]cid.Cid, sqrLn/2)
	for i := 0; i < sqrLn/2; i++ {
		cids[i] = MustAxisCID(rsmt2d.Row, i, hdr.DAH, hdr.Height())
	}

	square, err := rsmt2d.NewExtendedDataSquare(
		share.DefaultRSMT2DCodec(),
		wrapper.NewConstructor(uint64(sqrLn/2)), uint(sqrLn),
		share.Size,
	)
	if err != nil {
		return nil, err
	}

	blkCh := g.bget.GetBlocks(ctx, cids)
	for blk := range blkCh { // NOTE: GetBlocks handles ctx, so we don't have to
		axis, err := AxisFromBlock(blk)
		if err != nil {
			// NOTE: Should never error in fact, as Hasher already validated the block
			return nil, fmt.Errorf("converting block to Axis: %w", err)
		}

		for shrIdx, shr := range axis.AxisShares {
			err = square.SetCell(uint(axis.AxisIndex), uint(shrIdx), shr)
			if err != nil {
				panic(err) // this should never happen and if it is... something is really wrong
			}
		}
	}

	// TODO(@Wondertan): Figure out a way to avoid recompute of what has been already computed
	//  during verification in AxisHasher
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

	var cids []cid.Cid //nolint:prealloc// we don't know how many rows with needed namespace there are
	for rowIdx, rowRoot := range hdr.DAH.RowRoots {
		if ns.IsOutsideRange(rowRoot, rowRoot) {
			continue
		}

		cids = append(cids, MustDataCID(rowIdx, hdr.DAH, hdr.Height(), ns))
	}
	if len(cids) == 0 {
		return share.NamespacedShares{}, nil
	}

	datas := make([]*Data, 0, len(cids))
	blkCh := g.bget.GetBlocks(ctx, cids)
	for blk := range blkCh { // NOTE: GetBlocks handles ctx, so we don't have to
		data, err := DataFromBlock(blk)
		if err != nil {
			// NOTE: Should never error in fact, as Hasher already validated the block
			return nil, fmt.Errorf("converting block to Data: %w", err)
		}

		datas = append(datas, data)
	}

	slices.SortFunc(datas, func(a, b *Data) int {
		if a.DataID.AxisIndex < b.DataID.AxisIndex {
			return -1
		}
		return 1
	})

	nShrs := make(share.NamespacedShares, len(datas))
	for i, row := range datas {
		nShrs[i] = share.NamespacedRow{
			Shares: row.DataShares,
			Proof:  &row.DataProof,
		}
	}

	// NOTE: We don't need to call Verify here as Bitswap already did it for us internal.
	return nShrs, nil
}
