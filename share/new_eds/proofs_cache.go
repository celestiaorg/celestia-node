package eds

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"sync/atomic"

	"github.com/ipfs/boxo/blockservice"
	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"

	"github.com/celestiaorg/celestia-app/pkg/wrapper"
	"github.com/celestiaorg/nmt"
	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/ipld"
	"github.com/celestiaorg/celestia-node/share/shwap"
)

var _ AccessorStreamer = (*proofsCache)(nil)

// proofsCache is eds accessor that caches proofs for rows and columns. It also caches extended
// axis Shares. It is used to speed up the process of building proofs for rows and columns,
// reducing the number of reads from the underlying accessor.
type proofsCache struct {
	inner AccessorStreamer

	// lock protects axisCache
	lock sync.RWMutex
	// axisCache caches the axis Shares and proofs. Index in the slice corresponds to the axis type.
	// The map key is the index of the axis.
	axisCache []map[int]axisWithProofs
	// size caches the size of the data square
	size atomic.Int32
	// disableCache disables caching of rows for testing purposes
	disableCache bool
}

// axisWithProofs is used to cache the extended axis Shares and proofs.
type axisWithProofs struct {
	half AxisHalf
	// shares are the extended axis Shares
	shares []share.Share
	// root caches the root of the tree. It will be set only when proofs are calculated
	root []byte
	// proofs are stored in a blockservice.BlockGetter by their CID. It will be set only when proofs
	// are calculated and will be used to get the proof for a specific share. BlockGetter is used to
	// reuse ipld based proof generation logic, which traverses the tree from the root to the leafs and
	// collects the nodes on the path. This is temporary and will be replaced with a more efficient
	// proof caching mechanism in nmt package, once it is implemented.
	proofs blockservice.BlockGetter
}

// WithProofsCache creates a new eds accessor with caching of proofs for rows and columns. It is
// used to speed up the process of building proofs for rows and columns, reducing the number of
// reads from the underlying accessor.
func WithProofsCache(ac AccessorStreamer) AccessorStreamer {
	rows := make(map[int]axisWithProofs)
	cols := make(map[int]axisWithProofs)
	axisCache := []map[int]axisWithProofs{rows, cols}
	return &proofsCache{
		inner:     ac,
		axisCache: axisCache,
	}
}

func (c *proofsCache) Size(ctx context.Context) int {
	size := c.size.Load()
	if size == 0 {
		size = int32(c.inner.Size(ctx))
		c.size.Store(size)
	}
	return int(size)
}

func (c *proofsCache) Sample(ctx context.Context, rowIdx, colIdx int) (shwap.Sample, error) {
	axisType, axisIdx, shrIdx := rsmt2d.Row, rowIdx, colIdx
	ax, err := c.axisWithProofs(ctx, axisType, axisIdx)
	if err != nil {
		return shwap.Sample{}, err
	}

	// build share proof from proofs cached for given axis
	share := ax.shares[shrIdx]
	proofs, err := ipld.GetProof(ctx, ax.proofs, ax.root, shrIdx, c.Size(ctx))
	if err != nil {
		return shwap.Sample{}, fmt.Errorf("building proof from cache: %w", err)
	}

	return shwap.Sample{
		Share:     share,
		Proof:     &proofs,
		ProofType: axisType,
	}, nil
}

func (c *proofsCache) axisWithProofs(ctx context.Context, axisType rsmt2d.Axis, axisIdx int) (axisWithProofs, error) {
	// return axis with proofs from cache if possible
	ax, ok := c.getAxisFromCache(axisType, axisIdx)
	if ax.proofs != nil {
		// return axis with proofs from cache, only if proofs are already calculated
		return ax, nil
	}

	if !ok {
		// if shares are not in cache, read them from the inner accessor
		shares, err := c.axisShares(ctx, axisType, axisIdx)
		if err != nil {
			return axisWithProofs{}, fmt.Errorf("get axis: %w", err)
		}
		ax.shares = shares
	}

	// build proofs from Shares and cache them
	adder := ipld.NewProofsAdder(c.Size(ctx), true)
	tree := wrapper.NewErasuredNamespacedMerkleTree(
		uint64(c.Size(ctx)/2),
		uint(axisIdx),
		nmt.NodeVisitor(adder.VisitFn()),
	)
	for _, shr := range ax.shares {
		err := tree.Push(shr)
		if err != nil {
			return axisWithProofs{}, fmt.Errorf("push shares: %w", err)
		}
	}

	// build the tree
	root, err := tree.Root()
	if err != nil {
		return axisWithProofs{}, fmt.Errorf("calculating root: %w", err)
	}

	ax.root = root
	ax.proofs, err = newRowProofsGetter(adder.Proofs())
	if err != nil {
		return axisWithProofs{}, fmt.Errorf("creating proof getter: %w", err)
	}

	if !c.disableCache {
		c.storeAxisInCache(axisType, axisIdx, ax)
	}
	return ax, nil
}

func (c *proofsCache) AxisHalf(ctx context.Context, axisType rsmt2d.Axis, axisIdx int) (AxisHalf, error) {
	// return axis from cache if possible
	ax, ok := c.getAxisFromCache(axisType, axisIdx)
	if ok {
		return ax.half, nil
	}

	// read axis from inner accessor if axis is in the first quadrant
	half, err := c.inner.AxisHalf(ctx, axisType, axisIdx)
	if err != nil {
		return AxisHalf{}, fmt.Errorf("reading axis from inner accessor: %w", err)
	}

	if !c.disableCache {
		ax.half = half
		c.storeAxisInCache(axisType, axisIdx, ax)
	}

	return half, nil
}

func (c *proofsCache) RowNamespaceData(
	ctx context.Context,
	namespace share.Namespace,
	rowIdx int,
) (shwap.RowNamespaceData, error) {
	ax, err := c.axisWithProofs(ctx, rsmt2d.Row, rowIdx)
	if err != nil {
		return shwap.RowNamespaceData{}, err
	}

	row, proof, err := ipld.GetSharesByNamespace(ctx, ax.proofs, ax.root, namespace, c.Size(ctx))
	if err != nil {
		return shwap.RowNamespaceData{}, fmt.Errorf("shares by namespace %s for row %v: %w", namespace.String(), rowIdx, err)
	}

	return shwap.RowNamespaceData{
		Shares: row,
		Proof:  proof,
	}, nil
}

func (c *proofsCache) Shares(ctx context.Context) ([]share.Share, error) {
	odsSize := c.Size(ctx) / 2
	shares := make([]share.Share, 0, odsSize*odsSize)
	for i := 0; i < c.Size(ctx)/2; i++ {
		ax, err := c.AxisHalf(ctx, rsmt2d.Row, i)
		if err != nil {
			return nil, err
		}

		half := ax.Shares
		if ax.IsParity {
			shares, err = c.axisShares(ctx, rsmt2d.Row, i)
			if err != nil {
				return nil, err
			}
			half = shares[:odsSize]
		}

		shares = append(shares, half...)
	}
	return shares, nil
}

func (c *proofsCache) Reader() (io.Reader, error) {
	odsSize := c.Size(context.TODO()) / 2
	reader := NewSharesReader(odsSize, c.getShare)
	return reader, nil
}

func (c *proofsCache) Close() error {
	return c.inner.Close()
}

func (c *proofsCache) axisShares(ctx context.Context, axisType rsmt2d.Axis, axisIdx int) ([]share.Share, error) {
	ax, ok := c.getAxisFromCache(axisType, axisIdx)
	if ok && ax.shares != nil {
		return ax.shares, nil
	}

	half, err := c.AxisHalf(ctx, axisType, axisIdx)
	if err != nil {
		return nil, err
	}

	shares, err := half.Extended()
	if err != nil {
		return nil, fmt.Errorf("extending shares: %w", err)
	}

	if !c.disableCache {
		ax.shares = shares
		c.storeAxisInCache(axisType, axisIdx, ax)
	}
	return shares, nil
}

func (c *proofsCache) storeAxisInCache(axisType rsmt2d.Axis, axisIdx int, axis axisWithProofs) {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.axisCache[axisType][axisIdx] = axis
}

func (c *proofsCache) getAxisFromCache(axisType rsmt2d.Axis, axisIdx int) (axisWithProofs, bool) {
	c.lock.RLock()
	defer c.lock.RUnlock()
	ax, ok := c.axisCache[axisType][axisIdx]
	return ax, ok
}

func (c *proofsCache) getShare(rowIdx, colIdx int) ([]byte, error) {
	ctx := context.TODO()
	odsSize := c.Size(ctx) / 2
	half, err := c.AxisHalf(ctx, rsmt2d.Row, rowIdx)
	if err != nil {
		return nil, fmt.Errorf("reading axis half: %w", err)
	}

	// if share is from the same side of axis return share right away
	if colIdx > odsSize == half.IsParity {
		if half.IsParity {
			colIdx = colIdx - odsSize
		}
		return half.Shares[colIdx], nil
	}

	// if share index is from opposite part of axis, obtain full axis shares
	shares, err := c.axisShares(ctx, rsmt2d.Row, rowIdx)
	if err != nil {
		return nil, fmt.Errorf("reading axis shares: %w", err)
	}
	return shares[colIdx], nil
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
