package eds

import (
	"context"
	"errors"
	"fmt"

	"github.com/ipfs/boxo/blockservice"
	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"

	"github.com/celestiaorg/celestia-app/pkg/wrapper"
	"github.com/celestiaorg/nmt"
	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds/byzantine"
	"github.com/celestiaorg/celestia-node/share/ipld"
)

type CacheFile struct {
	File

	// TODO(@walldiss): add columns support
	rowCache map[int]inMemoryAxis
	// disableCache disables caching of rows for testing purposes
	disableCache bool
}

type inMemoryAxis struct {
	shares []share.Share
	proofs blockservice.BlockGetter
}

func NewCacheFile(f File) *CacheFile {
	return &CacheFile{
		File:     f,
		rowCache: make(map[int]inMemoryAxis),
	}
}

func (f *CacheFile) ShareWithProof(
	ctx context.Context,
	idx int,
	axis rsmt2d.Axis,
	axisRoot []byte,
) (*byzantine.ShareWithProof, error) {
	sqrLn := f.Size()
	axsIdx, shrIdx := idx/sqrLn, idx%sqrLn
	if axis == rsmt2d.Col {
		axsIdx, shrIdx = shrIdx, axsIdx
	}

	row := f.rowCache[axsIdx]
	if row.proofs == nil {
		shrs, err := f.Axis(axsIdx, axis)
		if err != nil {
			return nil, err
		}

		// calculate proofs
		adder := ipld.NewProofsAdder(sqrLn*2, ipld.CollectShares)
		tree := wrapper.NewErasuredNamespacedMerkleTree(uint64(sqrLn/2), uint(axsIdx),
			nmt.NodeVisitor(adder.VisitFn()))
		for _, shr := range shrs {
			err = tree.Push(shr)
			if err != nil {
				return nil, err
			}
		}

		if _, err := tree.Root(); err != nil {
			return nil, err
		}

		row = inMemoryAxis{
			shares: shrs,
			proofs: newRowProofsGetter(adder.Proofs()),
		}

		if !f.disableCache {
			f.rowCache[axsIdx] = row
		}
	}

	// TODO(@walldiss): find prealloc size for proofs
	proof := make([]cid.Cid, 0, 8)
	rootCid := ipld.MustCidFromNamespacedSha256(axisRoot)
	proofs, err := ipld.GetProof(ctx, row.proofs, rootCid, proof, shrIdx, sqrLn)
	if err != nil {
		return nil, fmt.Errorf("bulding proof from cache: %w", err)
	}

	return byzantine.NewShareWithProof(shrIdx, row.shares[shrIdx], proofs), nil
}

func (f *CacheFile) Axis(idx int, axis rsmt2d.Axis) ([]share.Share, error) {
	row, ok := f.rowCache[idx]
	if ok {
		return row.shares, nil
	}

	shrs, err := f.File.Axis(idx, axis)
	if err != nil {
		return nil, err
	}

	// cache row shares
	if !f.disableCache {
		f.rowCache[idx] = inMemoryAxis{
			shares: shrs,
		}
	}
	return shrs, nil
}

// TODO(@walldiss): needs to be implemented
func (f *CacheFile) EDS() (*rsmt2d.ExtendedDataSquare, error) {
	return f.File.EDS()
}

// rowProofsGetter implements blockservice.BlockGetter interface
type rowProofsGetter struct {
	proofs map[cid.Cid]blocks.Block
}

func newRowProofsGetter(rawProofs map[cid.Cid][]byte) *rowProofsGetter {
	proofs := make(map[cid.Cid]blocks.Block, len(rawProofs))
	for k, v := range rawProofs {
		proofs[k] = blocks.NewBlock(v)
	}
	return &rowProofsGetter{
		proofs: proofs,
	}
}

func (r rowProofsGetter) GetBlock(_ context.Context, c cid.Cid) (blocks.Block, error) {
	if b, ok := r.proofs[c]; ok {
		return b, nil
	}
	return nil, errors.New("block not found")
}

func (r rowProofsGetter) GetBlocks(_ context.Context, _ []cid.Cid) <-chan blocks.Block {
	panic("not implemented")
}
