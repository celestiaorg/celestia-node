package file

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/ipfs/boxo/blockservice"
	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"

	"github.com/celestiaorg/celestia-app/pkg/wrapper"
	"github.com/celestiaorg/nmt"
	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/ipld"
)

var _ EdsFile = (*CacheFile)(nil)

type CacheFile struct {
	EdsFile

	// lock protects axisCache
	lock sync.RWMutex
	// axisCache caches the axis shares and proofs
	axisCache []map[int]inMemoryAxis
	// disableCache disables caching of rows for testing purposes
	disableCache bool
}

type inMemoryAxis struct {
	shares []share.Share

	// root will be set only when proofs are calculated
	root   []byte
	proofs blockservice.BlockGetter
}

func NewCacheFile(f EdsFile) *CacheFile {
	return &CacheFile{
		EdsFile:   f,
		axisCache: []map[int]inMemoryAxis{make(map[int]inMemoryAxis), make(map[int]inMemoryAxis)},
	}
}

func (f *CacheFile) Share(ctx context.Context, x, y int) (*share.ShareWithProof, error) {
	axisType, axisIdx, shrIdx := rsmt2d.Row, y, x
	if x < f.Size()/2 && y >= f.Size()/2 {
		axisType, axisIdx, shrIdx = rsmt2d.Col, x, y
	}

	ax, err := f.axisWithProofs(ctx, axisType, axisIdx)
	if err != nil {
		return nil, err
	}

	// build share proof from proofs cached for given axis
	share, err := ipld.GetShareWithProof(ctx, ax.proofs, ax.root, shrIdx, f.Size(), axisType)
	if err != nil {
		return nil, fmt.Errorf("building proof from cache: %w", err)
	}

	return share, nil
}

func (f *CacheFile) axisWithProofs(ctx context.Context, axisType rsmt2d.Axis, axisIdx int) (inMemoryAxis, error) {
	// return axis with proofs from cache if possible
	ax, ok := f.getAxisFromCache(axisType, axisIdx)
	if ax.proofs != nil {
		return ax, nil
	}

	// build proofs from shares and cache them
	if !ok {
		shrs, err := f.axis(ctx, axisType, axisIdx)
		if err != nil {
			return inMemoryAxis{}, fmt.Errorf("get axis: %w", err)
		}
		ax.shares = shrs
	}

	// calculate proofs
	adder := ipld.NewProofsAdder(f.Size(), true)
	tree := wrapper.NewErasuredNamespacedMerkleTree(uint64(f.Size()/2), uint(axisIdx),
		nmt.NodeVisitor(adder.VisitFn()))
	for _, shr := range ax.shares {
		err := tree.Push(shr)
		if err != nil {
			return inMemoryAxis{}, fmt.Errorf("push shares: %w", err)
		}
	}

	// build the tree
	root, err := tree.Root()
	if err != nil {
		return inMemoryAxis{}, fmt.Errorf("calculating root: %w", err)
	}

	ax.root = root
	ax.proofs, err = newRowProofsGetter(adder.Proofs())
	if err != nil {
		return inMemoryAxis{}, fmt.Errorf("creating proof getter: %w", err)
	}

	if !f.disableCache {
		f.storeAxisInCache(axisType, axisIdx, ax)
	}
	return ax, nil
}

func (f *CacheFile) AxisHalf(ctx context.Context, axisType rsmt2d.Axis, axisIdx int) ([]share.Share, error) {
	// return axis from cache if possible
	ax, ok := f.getAxisFromCache(axisType, axisIdx)
	if ok {
		return ax.shares[:f.Size()/2], nil
	}

	// read axis from file if axis is in the first quadrant
	half, err := f.EdsFile.AxisHalf(ctx, axisType, axisIdx)
	if err != nil {
		return nil, fmt.Errorf("reading axis from inner file: %w", err)
	}

	if !f.disableCache {
		axis, err := extendShares(codec, half)
		if err != nil {
			return nil, fmt.Errorf("extending shares: %w", err)
		}
		ax.shares = axis
		f.storeAxisInCache(axisType, axisIdx, ax)
	}

	return half, nil
}

func (f *CacheFile) axis(ctx context.Context, axisType rsmt2d.Axis, axisIdx int) ([]share.Share, error) {
	original, err := f.AxisHalf(ctx, axisType, axisIdx)
	if err != nil {
		return nil, err
	}

	return extendShares(codec, original)
}

func (f *CacheFile) Data(ctx context.Context, namespace share.Namespace, rowIdx int) (share.NamespacedRow, error) {
	ax, err := f.axisWithProofs(ctx, rsmt2d.Row, rowIdx)
	if err != nil {
		return share.NamespacedRow{}, err
	}

	row, proof, err := ipld.GetSharesByNamespace(ctx, ax.proofs, ax.root, namespace, f.Size())
	if err != nil {
		return share.NamespacedRow{}, fmt.Errorf("shares by namespace %s for row %v: %w", namespace.String(), rowIdx, err)
	}

	return share.NamespacedRow{
		Shares: row,
		Proof:  proof,
	}, nil
}

func (f *CacheFile) EDS(ctx context.Context) (*rsmt2d.ExtendedDataSquare, error) {
	shares := make([][]byte, 0, f.Size()*f.Size())
	for i := 0; i < f.Size(); i++ {
		ax, err := f.axis(ctx, rsmt2d.Row, i)
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

func (f *CacheFile) storeAxisInCache(axisType rsmt2d.Axis, axisIdx int, axis inMemoryAxis) {
	f.lock.Lock()
	defer f.lock.Unlock()
	f.axisCache[axisType][axisIdx] = axis
}

func (f *CacheFile) getAxisFromCache(axisType rsmt2d.Axis, axisIdx int) (inMemoryAxis, bool) {
	f.lock.RLock()
	defer f.lock.RUnlock()
	ax, ok := f.axisCache[axisType][axisIdx]
	return ax, ok
}

// rowProofsGetter implements blockservice.BlockGetter interface
type rowProofsGetter struct {
	proofs map[cid.Cid]blocks.Block
}

func newRowProofsGetter(rawProofs map[cid.Cid][]byte) (*rowProofsGetter, error) {
	proofs := make(map[cid.Cid]blocks.Block, len(rawProofs))
	for k, v := range rawProofs {
		b, err := blocks.NewBlockWithCid(v, k)
		if err != nil {
			return nil, err
		}
		proofs[k] = b
	}
	return &rowProofsGetter{proofs: proofs}, nil
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
