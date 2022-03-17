package ipld

import (
	"context"
	"math"
	"math/rand"

	"github.com/ipfs/go-cid"
	ipld "github.com/ipfs/go-ipld-format"
	"golang.org/x/sync/errgroup"

	"github.com/tendermint/tendermint/pkg/da"
	"github.com/tendermint/tendermint/pkg/wrapper"

	"github.com/celestiaorg/celestia-node/ipld/plugin"
	"github.com/celestiaorg/nmt"
	"github.com/celestiaorg/nmt/namespace"
	"github.com/celestiaorg/rsmt2d"
)

// RetrieveData asynchronously fetches block data using the minimum number
// of requests to IPFS. It fails if one of the random samples sampled is not available.
func RetrieveData(
	parentCtx context.Context,
	dah *da.DataAvailabilityHeader,
	dag ipld.DAGService,
	codec rsmt2d.Codec,
) (*rsmt2d.ExtendedDataSquare, error) {
	edsWidth := len(dah.RowsRoots)
	dataSquare := make([][]byte, edsWidth*edsWidth)

	q := rand.Intn(4) + 1 //nolint:gosec
	quadrant, err := pickQuadrant(q, dah.RowsRoots)
	if err != nil {
		return nil, err
	}

	extended, err := repairDataSquare(parentCtx, quadrant, dah, dag, dataSquare, codec, true)
	if err == nil || err != rsmt2d.ErrUnrepairableDataSquare {
		return extended, err
	}

	// Trying one more time to repair dataSquare, using dah.Columns
	quadrant = nil
	quadrant, err = pickQuadrant(q, dah.ColumnRoots)
	if err != nil {
		return nil, err
	}

	return repairDataSquare(parentCtx, quadrant, dah, dag, dataSquare, codec, false)
}

type quadrant struct {
	isLeftSubtree bool
	from          int
	to            int
	rootCids      []cid.Cid
}

func repairDataSquare(
	parentCtx context.Context,
	quadrant *quadrant,
	dah *da.DataAvailabilityHeader,
	dag ipld.DAGService,
	dataSquare [][]byte,
	codec rsmt2d.Codec,
	isRow bool,
) (*rsmt2d.ExtendedDataSquare, error) {
	edsWidth := math.Sqrt(float64(len(dataSquare)))
	errGroup, ctx := errgroup.WithContext(parentCtx)

	errGroup.Go(func() error {
		return fillQuadrant(ctx, quadrant, dag, dataSquare, isRow)
	})
	if err := errGroup.Wait(); err != nil {
		return nil, err
	}

	batchAdder := NewNmtNodeAdder(parentCtx, ipld.NewBatch(parentCtx, dag, ipld.MaxSizeBatchOption(batchSize(int(edsWidth)))))
	tree := wrapper.NewErasuredNamespacedMerkleTree(uint64(edsWidth)/2, nmt.NodeVisitor(batchAdder.Visit))
	extended, err := rsmt2d.RepairExtendedDataSquare(dah.RowsRoots, dah.ColumnRoots, dataSquare, codec, tree.Constructor)
	if err != nil {
		return nil, err
	}

	return extended, batchAdder.Commit()
}

func pickQuadrant(qNumber int, roots [][]byte) (*quadrant, error) {
	edsWidth := len(roots)
	// get the random number from 1 to 4.
	quadrant := &quadrant{rootCids: make([]cid.Cid, edsWidth/2)}
	// quadrants 1 and 3 corresponds to left subtree,
	// 2 and 4 to the right subtree
	/*
		| 1 | 2 |
		| 3 | 4 |
	*/
	// choose subtree
	if qNumber%2 == 1 {
		quadrant.isLeftSubtree = true
	}
	// define range of shares for sampling
	if qNumber > 2 {
		quadrant.from = edsWidth / 2
		quadrant.to = edsWidth
	} else {
		quadrant.to = edsWidth / 2
	}

	var err error
	for index, counter := quadrant.from, 0; index < quadrant.to; index++ {
		quadrant.rootCids[counter], err = plugin.CidFromNamespacedSha256(roots[index])
		if err != nil {
			return nil, err
		}
		counter++
	}
	return quadrant, nil
}

// fillQuadrant fetches 1/4 of shares for the given root
func fillQuadrant(
	ctx context.Context,
	quadrant *quadrant,
	dag ipld.NodeGetter,
	dataSquare [][]byte,
	isRow bool,
) error {
	errGroup, ctx := errgroup.WithContext(ctx)
	for i := 0; i < len(quadrant.rootCids); i++ {
		i := i
		errGroup.Go(func() error {
			leaves, err := getSubtreeLeaves(
				ctx,
				quadrant.rootCids[i],
				dag,
				quadrant.isLeftSubtree,
				uint32(len(quadrant.rootCids)/2),
			)
			if err != nil {
				return err
			}
			quadrantWidth := len(quadrant.rootCids)
			for leafIdx, leaf := range leaves {
				// shares from the right subtree should be
				// inserted after all shares from the left subtree
				if !quadrant.isLeftSubtree {
					leafIdx += len(leaves)
				}
				shareData := leaf.RawData()[1:]
				// dataSquare represents a single dimensional slice
				// of 4 quadrants. The representation of quadrants in dataSquare will be
				// | 1 | | 2 | | 3 | | 4 |
				// position is calculated by offsetting the index to respective quadrant
				position := ((i + quadrant.from) * quadrantWidth * 2) + leafIdx
				if !isRow {
					// for columns position is computed by multyplying a rowIndex(leafIndex) with
					// quadrantWidth + colIndex + offset
					position = leafIdx*quadrantWidth*2 + i + quadrant.from
				}
				dataSquare[position] = shareData[NamespaceSize:]

			}
			return err
		})
	}
	return errGroup.Wait()
}

// getSubtreeLeaves returns only one subtree - left or right
func getSubtreeLeaves(
	ctx context.Context,
	root cid.Cid,
	dag ipld.NodeGetter,
	isLeftSubtree bool,
	treeSize uint32,
) ([]ipld.Node, error) {
	nd, err := dag.Get(ctx, root)
	if err != nil {
		return nil, err
	}
	links := nd.Links()
	subtreeRootHash := links[0].Cid
	if !isLeftSubtree && len(links) > 1 {
		subtreeRootHash = links[1].Cid
	}

	return getLeaves(ctx, dag, subtreeRootHash, make([]ipld.Node, 0, treeSize))
}

// getLeaves recursively starts going down to find all leafs from the given root
func getLeaves(ctx context.Context, dag ipld.NodeGetter, root cid.Cid, leaves []ipld.Node) ([]ipld.Node, error) {
	// request the node
	nd, err := dag.Get(ctx, root)
	if err != nil {
		return nil, err
	}

	// look for links
	lnks := nd.Links()
	if len(lnks) == 1 {
		// in case there is only one we reached tree's bottom, so finally request the leaf.
		nd, err = dag.Get(ctx, lnks[0].Cid)
		if err != nil {
			return nil, err
		}

		return append(leaves, nd), nil
	}

	for _, node := range lnks {
		// recursively walk down through selected children to get the leaves
		leaves, err = getLeaves(ctx, dag, node.Cid, leaves)
		if err != nil {
			return nil, err
		}
	}
	return leaves, nil
}

// GetLeafData fetches and returns the data for leaf leafIndex of root rootCid.
// It stops and returns an error if the provided context is canceled before
// finishing
func GetLeafData(
	ctx context.Context,
	rootCid cid.Cid,
	leafIndex uint32,
	totalLeafs uint32, // this corresponds to the extended square width
	dag ipld.NodeGetter,
) ([]byte, error) {
	nd, err := GetLeaf(ctx, dag, rootCid, int(leafIndex), int(totalLeafs))
	if err != nil {
		return nil, err
	}

	return nd.RawData()[1:], nil
}

// GetLeaf fetches and returns the raw leaf.
// It walks down the IPLD NMT tree until it finds the requested one.
func GetLeaf(ctx context.Context, dag ipld.NodeGetter, root cid.Cid, leaf, total int) (ipld.Node, error) {
	// request the node
	nd, err := dag.Get(ctx, root)
	if err != nil {
		return nil, err
	}

	// look for links
	lnks := nd.Links()
	if len(lnks) == 1 {
		// in case there is only one we reached tree's bottom, so finally request the leaf.
		return dag.Get(ctx, lnks[0].Cid)
	}

	// route walk to appropriate children
	total /= 2 // as we are using binary tree, every step decreases total leaves in a half
	if leaf < total {
		root = lnks[0].Cid // if target leave on the left, go with walk down the first children
	} else {
		root, leaf = lnks[1].Cid, leaf-total // otherwise go down the second
	}

	// recursively walk down through selected children
	return GetLeaf(ctx, dag, root, leaf, total)
}

// GetLeavesByNamespace returns all the shares from the given DataAvailabilityHeader root
// with the given namespace.ID.
func GetLeavesByNamespace(
	ctx context.Context,
	dag ipld.NodeGetter,
	root cid.Cid,
	nID namespace.ID,
) (out []ipld.Node, err error) {
	rootH := plugin.NamespacedSha256FromCID(root)
	if nID.Less(nmt.MinNamespace(rootH, nID.Size())) || !nID.LessOrEqual(nmt.MaxNamespace(rootH, nID.Size())) {
		return nil, ErrNotFoundInRange
	}
	// request the node
	nd, err := dag.Get(ctx, root)
	if err != nil {
		return
	}
	// check links
	lnks := nd.Links()
	if len(lnks) == 1 {
		// if there is one link, then this is a leaf node, so just return it
		out = append(out, nd)
		return
	}
	// if there are some links, then traverse them
	for _, lnk := range nd.Links() {
		nds, err := GetLeavesByNamespace(ctx, dag, lnk.Cid, nID)
		if err != nil {
			if err == ErrNotFoundInRange {
				// There is always right and left child and it is ok if one of them does not have a required nID.
				continue
			}
		}
		out = append(nds, out...)
	}
	return out, err
}
