package ipld

import (
	"context"
	"errors"

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

type totalShares struct {
	data [][]byte
	err  chan error
}

// RetrieveData asynchronously fetches block data using the minimum number
// of requests to IPFS. It fails if one of the random samples sampled is not available.
func RetrieveData(
	parentCtx context.Context,
	dah *da.DataAvailabilityHeader,
	dag ipld.NodeGetter,
	codec rsmt2d.Codec,
) (*rsmt2d.ExtendedDataSquare, error) {
	edsWidth := len(dah.RowsRoots)
	rowRoots := dah.RowsRoots
	colRoots := dah.ColumnRoots
	shares := &totalShares{
		data: make([][]byte, edsWidth*edsWidth),
		err:  make(chan error),
	}

	ctx, cancel := context.WithCancel(parentCtx)
	defer cancel()

	go fillBlockWithData(ctx, rowRoots, dag, true, shares)
	go fillBlockWithData(ctx, colRoots, dag, false, shares)

	for i := 0; i < len(dah.RowsRoots); i++ {
		select {
		case err := <-shares.err:
			if err != nil {
				return nil, err
			}
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	tree := wrapper.NewErasuredNamespacedMerkleTree(uint64(edsWidth) / 2)
	return rsmt2d.RepairExtendedDataSquare(rowRoots, colRoots, shares.data, codec, tree.Constructor)
}

// fillBlockWithData fetches 1/4 of shares for the given root
func fillBlockWithData(
	ctx context.Context,
	data [][]byte, dag ipld.NodeGetter,
	isRow bool,
	f *totalShares,
) {
	for i := 0; i < len(data)/2; i++ {
		go func(i int) {
			rootHash, err := plugin.CidFromNamespacedSha256(data[i])
			if err != nil {
				f.err <- err
				return
			}
			subtreeRootHash, err := getHalfTreeNodeHash(ctx, rootHash, dag, true)
			if err != nil {
				f.err <- err
				return
			}

			leafs, err := GetLeafs(ctx, dag, *subtreeRootHash)
			if err != nil {
				f.err <- err
				return
			}
			for leafIdx, leaf := range leafs {
				shareData := leaf.RawData()[1:]
				// it's not needed to store data for cols
				// as we are fetching data from the same share for rows and cols
				if isRow {
					f.data[(i*len(data))+leafIdx] = shareData[NamespaceSize:]
				}
			}
			f.err <- nil
		}(i)
	}
}

// getHalfTreeNodeHash returns only one subtree - left or right
func getHalfTreeNodeHash(ctx context.Context, root cid.Cid, dag ipld.NodeGetter, isLeftSubtree bool) (*cid.Cid, error) {
	nd, err := dag.Get(ctx, root)
	if err != nil {
		return nil, err
	}
	links := nd.Links()
	if isLeftSubtree || len(links) == 1 {
		return &links[0].Cid, nil
	}

	return &links[1].Cid, nil
}

// GetLeafs recursively starts going down to find all leafs from the given root
func GetLeafs(ctx context.Context, dag ipld.NodeGetter, root cid.Cid) (leafs []ipld.Node, err error) {
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
		leafs = append(leafs, nd)
		return
	}

	for _, node := range lnks {
		// recursively walk down through selected children to get the leafs
		l, err := GetLeafs(ctx, dag, node.Cid)
		if err != nil {
			return nil, err
		}

		leafs = append(leafs, l...)
	}
	return
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
