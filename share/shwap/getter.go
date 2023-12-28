package shwap

import (
	"context"
	"fmt"
	"sync"

	"github.com/ipfs/boxo/blockstore"
	"github.com/ipfs/boxo/exchange"
	block "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"

	"github.com/celestiaorg/celestia-app/pkg/wrapper"
	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/share"
)

// TODO: GetRow method
type Getter struct {
	fetch  exchange.SessionExchange
	bstore blockstore.Blockstore
}

func NewGetter(fetch exchange.SessionExchange, bstore blockstore.Blockstore) *Getter {
	return &Getter{fetch: fetch, bstore: bstore}
}

// TODO: Make GetSamples so it provides proofs to users.
// GetShares fetches in the Block/EDS by their indexes.
// Automatically caches them on the Blockstore.
// Guarantee that the returned shares are in the same order as shrIdxs.
func (g *Getter) GetShares(ctx context.Context, hdr *header.ExtendedHeader, smplIdxs ...int) ([]share.Share, error) {
	sids := make([]SampleID, len(smplIdxs))
	for i, shrIdx := range smplIdxs {
		sid, err := NewSampleID(hdr.Height(), shrIdx, hdr.DAH)
		if err != nil {
			return nil, err
		}

		sids[i] = sid
	}

	smplsMu := sync.Mutex{}
	smpls := make(map[int]Sample, len(smplIdxs))
	verifyFn := func(s Sample) error {
		err := s.Verify(hdr.DAH)
		if err != nil {
			return err
		}

		smplIdx := int(s.SampleID.RowIndex)*len(hdr.DAH.RowRoots) + int(s.SampleID.ShareIndex)
		smplsMu.Lock()
		smpls[smplIdx] = s
		smplsMu.Unlock()
		return nil
	}

	cids := make([]cid.Cid, len(smplIdxs))
	for i, sid := range sids {
		sampleVerifiers.Add(sid, verifyFn)
		cids[i] = sid.Cid()
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	ses := g.fetch.NewSession(ctx)
	// must start getting only after verifiers are registered
	blkCh, err := ses.GetBlocks(ctx, cids)
	if err != nil {
		return nil, fmt.Errorf("fetching blocks: %w", err)
	}
	// GetBlocks handles ctx and closes blkCh, so we don't have to
	blks := make([]block.Block, 0, len(smplIdxs))
	for blk := range blkCh {
		blks = append(blks, blk)
	}
	// only persist when all samples received
	if len(blks) != len(smplIdxs) {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("not all shares were found")
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
	shrs := make([]share.Share, len(smplIdxs))
	for i, smplIdx := range smplIdxs {
		shrs[i] = smpls[smplIdx].SampleShare
	}

	return shrs, nil
}

// GetEDS
// TODO(@Wondertan): Consider requesting randomized rows instead of ODS only
func (g *Getter) GetEDS(ctx context.Context, hdr *header.ExtendedHeader) (*rsmt2d.ExtendedDataSquare, error) {
	sqrLn := len(hdr.DAH.RowRoots)
	rids := make([]RowID, sqrLn/2)
	for i := 0; i < sqrLn/2; i++ {
		rid, err := NewRowID(hdr.Height(), uint16(i), hdr.DAH)
		if err != nil {
			return nil, err
		}

		rids[i] = rid
	}

	square, err := rsmt2d.NewExtendedDataSquare(
		share.DefaultRSMT2DCodec(),
		wrapper.NewConstructor(uint64(sqrLn/2)), uint(sqrLn),
		share.Size,
	)
	if err != nil {
		return nil, err
	}

	verifyFn := func(row Row) error {
		err := row.Verify(hdr.DAH)
		if err != nil {
			return err
		}

		for shrIdx, shr := range row.RowShares {
			err = square.SetCell(uint(row.RowIndex), uint(shrIdx), shr) // no synchronization needed
			if err != nil {
				panic(err) // this should never happen and if it is... something is really wrong
			}
		}

		return nil
	}

	cids := make([]cid.Cid, sqrLn/2)
	for i, rid := range rids {
		rowVerifiers.Add(rid, verifyFn)
		cids[i] = rid.Cid()
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	ses := g.fetch.NewSession(ctx)
	// must start getting only after verifiers are registered
	blkCh, err := ses.GetBlocks(ctx, cids)
	if err != nil {
		return nil, fmt.Errorf("fetching blocks: %w", err)
	}
	// GetBlocks handles ctx by closing blkCh, so we don't have to
	for range blkCh { //nolint:revive // it complains on empty block, but the code is functional
		// we handle writes in verifyFn so just wait for as many results as possible
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

	var dids []DataID //nolint:prealloc// we don't know how many rows with needed namespace there are
	for rowIdx, rowRoot := range hdr.DAH.RowRoots {
		if ns.IsOutsideRange(rowRoot, rowRoot) {
			continue
		}

		did, err := NewDataID(hdr.Height(), uint16(rowIdx), ns, hdr.DAH)
		if err != nil {
			return nil, err
		}

		dids = append(dids, did)
	}
	if len(dids) == 0 {
		return share.NamespacedShares{}, nil
	}

	datas := make([]Data, len(dids))
	verifyFn := func(d Data) error {
		err := d.Verify(hdr.DAH)
		if err != nil {
			return err
		}

		nsStartIdx := dids[0].RowIndex
		idx := d.RowIndex - nsStartIdx
		datas[idx] = d
		return nil
	}

	cids := make([]cid.Cid, len(dids))
	for i, did := range dids {
		dataVerifiers.Add(did, verifyFn)
		cids[i] = did.Cid()
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	ses := g.fetch.NewSession(ctx)
	// must start getting only after verifiers are registered
	blkCh, err := ses.GetBlocks(ctx, cids)
	if err != nil {
		return nil, fmt.Errorf("fetching blocks:%w", err)
	}
	// GetBlocks handles ctx by closing blkCh, so we don't have to
	for range blkCh { //nolint:revive // it complains on empty block, but the code is functional
		// we handle writes in verifyFn so just wait for as many results as possible
	}

	nShrs := make([]share.NamespacedRow, 0, len(datas))
	for _, row := range datas {
		proof := row.DataProof
		nShrs = append(nShrs, share.NamespacedRow{
			Shares: row.DataShares,
			Proof:  &proof,
		})
	}

	return nShrs, nil
}
