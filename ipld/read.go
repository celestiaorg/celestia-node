package ipld

import (
	"context"
	"errors"
	"fmt"
	"math/rand"

	"github.com/ipfs/go-cid"
	ipld "github.com/ipfs/go-ipld-format"

	"github.com/tendermint/tendermint/pkg/da"
	"github.com/tendermint/tendermint/pkg/wrapper"

	"github.com/celestiaorg/celestia-node/ipld/plugin"
	"github.com/celestiaorg/nmt"
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

func RetrieveDataExt(
	ctx context.Context,
	dah *da.DataAvailabilityHeader,
	dag ipld.NodeGetter,
	codec rsmt2d.Codec,
) (*rsmt2d.ExtendedDataSquare, error) {
	edsWidth := len(dah.RowsRoots)
	rowRoots := dah.RowsRoots
	colRoots := dah.ColumnRoots

	sc := newshareCounter(ctx, uint32(edsWidth))
	go fillTheBlock(ctx, rowRoots, true, dag, sc)
	go fillTheBlock(ctx, colRoots, false, dag, sc)
	err := sc.wait()
	if err != nil {
		fmt.Println(err)
		return nil, err
	}
	// flatten the square
	flattened := sc.flatten()

	tree := wrapper.NewErasuredNamespacedMerkleTree(uint64(edsWidth) / 2)
	return rsmt2d.RepairExtendedDataSquare(rowRoots, colRoots, flattened, codec, tree.Constructor)
}

func fillTheBlock(ctx context.Context, data [][]byte, isRow bool, dag ipld.NodeGetter, sc *shareCounter) {
	for _, number := range uniqueRandNumbers(int(sc.edsWidth/2), int(sc.edsWidth)) {
		go func(number uint32) {
			rootCid, _ := plugin.CidFromNamespacedSha256(data[number])
			leafs, _ := GetLeafsExt(ctx, dag, rootCid, int(sc.edsWidth/8))
			for leafIndex, l := range leafs {
				data := l.RawData()[1:]
				if isRow {
					sc.shareChan <- indexedShare{data: data[NamespaceSize:], index: index{row: uint32(number), col: uint32(leafIndex)}}
				} else {
					sc.shareChan <- indexedShare{data: data[NamespaceSize:], index: index{row: uint32(leafIndex), col: uint32(number)}}
				}
			}
		}(number)
	}
}

func GetLeafsExt(
	ctx context.Context,
	dag ipld.NodeGetter,
	root cid.Cid,
	maxDepth int,
) (leafs []ipld.Node, err error) {
	nd, err := dag.Get(ctx, root)
	if err != nil {
		return nil, err
	}
	if maxDepth == 0 {
		leafs = append(leafs, nd)
		return
	}
	links := nd.Links()
	if len(links) == 1 {
		leafs = append(leafs, nd)
		return
	}
	newDep := maxDepth / 2
	for _, leaf := range links {
		l, err := GetLeafsExt(ctx, dag, leaf.Cid, newDep)
		if err != nil {
			return nil, err
		}

		leafs = append(leafs, l...)
	}
	return leafs, nil
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
