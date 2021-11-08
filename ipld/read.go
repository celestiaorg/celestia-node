package ipld

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"reflect"

	"github.com/ipfs/go-cid"
	ipld "github.com/ipfs/go-ipld-format"

	"github.com/celestiaorg/celestia-core/pkg/da"
	"github.com/celestiaorg/celestia-core/pkg/wrapper"
	"github.com/celestiaorg/celestia-node/ipld/plugin"
	"github.com/celestiaorg/nmt/namespace"
	"github.com/celestiaorg/rsmt2d"
)

var ErrRetrieveTimeout = errors.New("retrieve data timeout")

// RetrieveData asynchronously fetches block data using the minimum number
// of requests to IPFS. It fails if one of the random samples sampled is not available.
func RetrieveData(
	ctx context.Context,
	dah *da.DataAvailabilityHeader,
	dag ipld.NodeGetter,
	codec rsmt2d.Codec,
) (*rsmt2d.ExtendedDataSquare, error) {
	edsWidth := len(dah.RowsRoots)
	sc := newshareCounter(ctx, uint32(edsWidth))
	// convert the row and col roots into Cids
	rowRoots := dah.RowsRoots
	colRoots := dah.ColumnRoots
	// sample 1/4 of the total extended square by sampling half of the leaves in
	// half of the rows.
	// Sampling is done randomly here in an effort to increase
	// resilience when the downloading node is also serving the shares back to
	// the network. // TODO @renaynay: this should probably eventually change to
	// just downloading the first half.
	for _, row := range uniqueRandNumbers(edsWidth/2, edsWidth) {
		for _, col := range uniqueRandNumbers(edsWidth/2, edsWidth) {
			rootCid, err := plugin.CidFromNamespacedSha256(rowRoots[row])
			if err != nil {
				return nil, err
			}

			go sc.retrieveShare(rootCid, true, row, col, dag)
		}
	}
	// wait until enough data has been collected, too many errors encountered,
	// or the timeout is reached
	err := sc.wait()
	if err != nil {
		return nil, err
	}
	// flatten the square
	flattened := sc.flatten()
	// repair the square
	tree := wrapper.NewErasuredNamespacedMerkleTree(uint64(edsWidth) / 2)
	return rsmt2d.RepairExtendedDataSquare(rowRoots, colRoots, flattened, codec, tree.Constructor)
}

// uniqueRandNumbers generates count unique random numbers with a max of max
func uniqueRandNumbers(count, max int) []uint32 {
	if count > max {
		panic(fmt.Sprintf("cannot create %d unique samples from a max of %d", count, max))
	}
	samples := make(map[uint32]struct{}, count)
	for i := 0; i < count; {
		// nolint:gosec // G404: Use of weak random number generator
		sample := uint32(rand.Intn(max))
		if _, has := samples[sample]; has {
			continue
		}
		samples[sample] = struct{}{}
		i++
	}
	out := make([]uint32, count)
	counter := 0
	for s := range samples {
		out[counter] = s
		counter++
	}
	return out
}

type index struct {
	row uint32
	col uint32
}

type indexedShare struct {
	data []byte
	index
}

// shareCounter is a thread safe tallying mechanism for share retrieval
type shareCounter struct {
	// all shares
	shares map[index][]byte
	// number of shares successfully collected
	counter uint32
	// the width of the extended data square
	edsWidth uint32
	// the minimum shares needed to repair the extended data square
	minSharesNeeded uint32

	shareChan chan indexedShare
	ctx       context.Context
	cancel    context.CancelFunc
	// any errors encountered when attempting to retrieve shares
	errc chan error
}

func newshareCounter(parentCtx context.Context, edsWidth uint32) *shareCounter {
	ctx, cancel := context.WithCancel(parentCtx)

	// calculate the min number of shares needed to repair the square
	minSharesNeeded := edsWidth * edsWidth / 4

	return &shareCounter{
		shares:          make(map[index][]byte),
		edsWidth:        edsWidth,
		minSharesNeeded: minSharesNeeded,
		shareChan:       make(chan indexedShare, 512),
		errc:            make(chan error, 1),
		ctx:             ctx,
		cancel:          cancel,
	}
}

// retrieveLeaf uses GetLeafData to fetch a single leaf and counts that leaf
func (sc *shareCounter) retrieveShare(
	rootCid cid.Cid,
	isRow bool,
	axisIdx uint32,
	idx uint32,
	dag ipld.NodeGetter,
) {
	data, err := GetLeafData(sc.ctx, rootCid, idx, sc.edsWidth, dag)
	if err != nil {
		select {
		case <-sc.ctx.Done():
		case sc.errc <- err:
		}
	}

	if len(data) < plugin.ShareSize {
		return
	}

	// switch the row and col indexes if needed
	rowIdx := idx
	colIdx := axisIdx
	if isRow {
		rowIdx = axisIdx
		colIdx = idx
	}

	select {
	case <-sc.ctx.Done():
	case sc.shareChan <- indexedShare{data: data[NamespaceSize:], index: index{row: rowIdx, col: colIdx}}:
	}
}

// wait until enough data has been collected, the timeout has been reached, or
// too many errors are encountered
func (sc *shareCounter) wait() error {
	defer sc.cancel()

	for {
		select {
		case <-sc.ctx.Done():
			err := sc.ctx.Err()
			if err == context.DeadlineExceeded {
				return ErrRetrieveTimeout
			}
			return err
		case share := <-sc.shareChan:
			_, has := sc.shares[share.index]
			// add iff it does not already exists
			if !has {
				sc.shares[share.index] = share.data
				sc.counter++
				// check finishing condition
				if sc.counter >= sc.minSharesNeeded {
					return nil
				}
			}

		case err := <-sc.errc:
			return fmt.Errorf("failure to retrieve data square: %w", err)
		}
	}
}

func (sc *shareCounter) flatten() [][]byte {
	flattended := make([][]byte, sc.edsWidth*sc.edsWidth)
	for index, data := range sc.shares {
		flattended[(index.row*sc.edsWidth)+index.col] = data
	}
	return flattended
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

// RowRootsByNamespaceID finds the row root(s) that contain the given namespace ID.
func RowRootsByNamespaceID(nID namespace.ID, dah *da.DataAvailabilityHeader) ([][]byte, error) {
	roots := make([][]byte, 0)
	for _, row := range dah.RowsRoots {
		// if nID exists within range of min -> max of row, return the row
		if !nID.Less(plugin.RowMin(row)) && nID.LessOrEqual(plugin.RowMax(row)) {
			roots = append(roots, row)
		}
	}
	if len(roots) > 0 {
		return roots, nil
	}
	// return min or max index depending on if nID is below the minimum namespace ID or exceeds the maximum
	// namespace ID of the given DataAvailabilityHeader
	if nID.Less(plugin.RowMin(dah.RowsRoots[0])) {
		return [][]byte{}, ErrBelowRange
	} else if !nID.LessOrEqual(plugin.RowMax(dah.RowsRoots[len(dah.RowsRoots)-1])) {
		max := dah.RowsRoots[len(dah.RowsRoots)-1]
		// TODO @renaynay: still need to figure out what kind of info to return as a part of the err
		return [][]byte{max}, ErrExceedsRange
	}
	return roots, ErrNotFoundInRange
}

func GetSharesByNamespace(
	ctx context.Context,
	nID namespace.ID,
	dah *da.DataAvailabilityHeader,
	rowRoots [][]byte,
	dagServ ipld.DAGService) ([][]byte, error) {
	shares := make([][]byte, 0)

	for _, rowRoot := range rowRoots {
		isLastRow := reflect.DeepEqual(dah.RowsRoots[len(dah.RowsRoots)-1], rowRoot)
		// compute the root CID from the DAH
		rootCid, err := plugin.CidFromNamespacedSha256(rowRoot)
		if err != nil {
			return shares, err
		}

		startingIndex, err := findStartingIndex(ctx, nID, dah, rootCid, dagServ, make([]string, 0))
		if err != nil {
			if isLastRow {
				if len(shares) > 0 {
					return shares, nil
				}
				return shares, err
			}
			continue
		}
		for {
			leaf, err := GetLeafData(ctx, rootCid, startingIndex, uint32(len(dah.RowsRoots)), dagServ)
			if err != nil {
				return shares, err
			}
			if nID.Equal(leaf[:8]) {
				shares = append(shares, leaf)
				startingIndex++
			} else {
				break
			}
		}
	}
	return shares, nil
}

// findStartingIndex returns the CID of the first leaf that contains the given nID
// in the tree corresponding to the given root CID.
// TODO @renaynay: wrap errors for this function
func findStartingIndex(
	ctx context.Context,
	nID namespace.ID,
	dah *da.DataAvailabilityHeader,
	rootCid cid.Cid,
	dag ipld.DAGService,
	currentPath []string) (uint32, error) {
	treeDepth := int(math.Log2(float64(len(dah.RowsRoots))))

	if isLeafLevel(treeDepth, len(currentPath)) {
		return startIndexFromPath(currentPath), nil
	}

	left := "0"
	right := "1"

	rootNode, err := dag.Get(ctx, rootCid)
	if err != nil {
		return 0, err
	}

	children := rootNode.Links()
	leftCID := children[0].Cid
	rightCID := children[1].Cid

	leftDigest := intervalDigestFromNamespaceHash(plugin.NamespacedSha256FromCID(leftCID))
	rightDigest := intervalDigestFromNamespaceHash(plugin.NamespacedSha256FromCID(rightCID))

	switch {
	case intervalContains(nID, leftDigest):
		// keep going left
		return findStartingIndex(ctx, nID, dah, leftCID, dag, append(currentPath, left))
	case intervalContains(nID, rightDigest):
		// keep going right
		return findStartingIndex(ctx, nID, dah, rightCID, dag, append(currentPath, right))
	default:
		return 0, fmt.Errorf("namespace ID %x within range of namespace IDs in tree, "+
			"but does not exist", nID.String()) // TODO @renaynay: better err
	}
}

// startIndexFromPath gets the starting leaf index from the given
// path
func startIndexFromPath(path []string) uint32 {
	start := 0
	indices := make([]int, 0)
	totalLeaves := math.Pow(2, float64(len(path)))
	for i := 0; i < int(totalLeaves); i++ {
		indices = append(indices, i)
	}

	for _, pos := range path {
		if pos == "0" {
			if len(indices) == 2 {
				return uint32(indices[0])
			}
			indices = indices[0 : len(indices)/2]
		} else {
			if len(indices) == 2 {
				return uint32(indices[1])
			}
			indices = indices[len(indices)/2:]
		}
	}
	return uint32(start)
}

func isLeafLevel(treeDepth, currentDepth int) bool {
	return treeDepth == currentDepth
}

// digest is a format for storing a node's minimum and
// maximum namespaceID
type digest struct {
	min namespace.ID
	max namespace.ID
}

func intervalDigestFromNamespaceHash(hash []byte) digest {
	return digest{
		min: plugin.RowMin(hash),
		max: plugin.RowMax(hash),
	}
}

func intervalContains(nID namespace.ID, digest digest) bool {
	return !nID.Less(digest.min) && nID.LessOrEqual(digest.max)
}
