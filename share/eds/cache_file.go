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

// TODO: allow concurrency safety fpr CacheFile methods
type CacheFile struct {
	File

	codec     rsmt2d.Codec
	axisCache []map[int]inMemoryAxis
	// disableCache disables caching of rows for testing purposes
	disableCache bool
}

type inMemoryAxis struct {
	shares []share.Share
	proofs blockservice.BlockGetter
}

func NewCacheFile(f File, codec rsmt2d.Codec) *CacheFile {
	return &CacheFile{
		File:      f,
		codec:     codec,
		axisCache: []map[int]inMemoryAxis{make(map[int]inMemoryAxis), make(map[int]inMemoryAxis)},
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

	ax, err := f.axisWithProofs(axsIdx, axis)
	if err != nil {
		return nil, err
	}

	// TODO(@walldiss): add proper calc to prealloc size for proofs
	proof := make([]cid.Cid, 0, 16)
	rootCid := ipld.MustCidFromNamespacedSha256(axisRoot)
	proofs, err := ipld.GetProof(ctx, ax.proofs, rootCid, proof, shrIdx, sqrLn)
	if err != nil {
		return nil, fmt.Errorf("bulding proof from cache: %w", err)
	}

	return byzantine.NewShareWithProof(shrIdx, ax.shares[shrIdx], proofs), nil
}

func (f *CacheFile) axisWithProofs(idx int, axis rsmt2d.Axis) (inMemoryAxis, error) {
	ax := f.axisCache[axis][idx]
	if ax.proofs != nil {
		return ax, nil
	}

	// build proofs from shares and cache them
	shrs, err := f.Axis(idx, axis)
	if err != nil {
		return inMemoryAxis{}, err
	}

	fmt.Println("building proofs for axis", idx, axis)
	// calculate proofs
	adder := ipld.NewProofsAdder(f.Size(), ipld.CollectShares)
	tree := wrapper.NewErasuredNamespacedMerkleTree(uint64(f.Size()/2), uint(idx),
		nmt.NodeVisitor(adder.VisitFn()))
	for _, shr := range shrs {
		err = tree.Push(shr)
		if err != nil {
			return inMemoryAxis{}, err
		}
	}

	// build the tree
	if _, err := tree.Root(); err != nil {
		return inMemoryAxis{}, err
	}

	ax = f.axisCache[axis][idx]
	ax.proofs = newRowProofsGetter(adder.Proofs())

	if !f.disableCache {
		f.axisCache[axis][idx] = ax
	}
	return ax, nil
}

func (f *CacheFile) Axis(idx int, axis rsmt2d.Axis) ([]share.Share, error) {
	// return axis from cache if possible
	ax, ok := f.axisCache[axis][idx]
	if ok {
		return ax.shares, nil
	}

	// recompute axis from half
	original, err := f.AxisHalf(idx, axis)
	if err != nil {
		return nil, err
	}

	parity, err := f.codec.Encode(original)
	if err != nil {
		return nil, err
	}

	shares := make([]share.Share, 0, len(original)+len(parity))
	shares = append(shares, original...)
	shares = append(shares, parity...)

	// cache axis shares
	if !f.disableCache {
		f.axisCache[axis][idx] = inMemoryAxis{
			shares: shares,
		}
	}
	return shares, nil
}

func (f *CacheFile) AxisHalf(idx int, axis rsmt2d.Axis) ([]share.Share, error) {
	// return axis from cache if possible
	ax, ok := f.axisCache[axis][idx]
	if ok {
		return ax.shares[:f.Size()/2], nil
	}

	// read axis from file if axis is in the first quadrant
	if idx < f.Size()/2 {
		return f.File.AxisHalf(idx, axis)
	}

	shares := make([]share.Share, 0, f.Size()/2)
	// extend opposite half of the square while collecting shares for the first half of required axis
	//TODO: parallelize this
	for i := 0; i < f.Size()/2; i++ {
		ax, err := f.Axis(i, oppositeAxis(axis))
		if err != nil {
			return nil, err
		}
		shares = append(shares, ax[idx])
	}
	return shares, nil
}

func (f *CacheFile) EDS() (*rsmt2d.ExtendedDataSquare, error) {
	shares := make([][]byte, 0, f.Size()*f.Size())
	for i := 0; i < f.Size(); i++ {
		ax, err := f.Axis(i, rsmt2d.Row)
		if err != nil {
			return nil, err
		}
		shares = append(shares, ax...)
	}

	eds, err := rsmt2d.ImportExtendedDataSquare(
		shares,
		share.DefaultRSMT2DCodec(),
		wrapper.NewConstructor(uint64(f.Size())/2))
	if err != nil {
		return nil, fmt.Errorf("recomputing data square: %w", err)
	}
	return eds, nil
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

func oppositeAxis(axis rsmt2d.Axis) rsmt2d.Axis {
	if axis == rsmt2d.Col {
		return rsmt2d.Row
	}
	return rsmt2d.Col
}
