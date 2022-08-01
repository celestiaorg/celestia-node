package ipld

import (
	"context"
	"sort"
	"sync"

	"github.com/ipfs/go-blockservice"
	"github.com/ipfs/go-cid"
	ipld "github.com/ipfs/go-ipld-format"

	"github.com/celestiaorg/celestia-node/ipld/plugin"
	"github.com/celestiaorg/nmt"

	"github.com/celestiaorg/nmt/namespace"
)

// GetShare fetches and returns the data for leaf `leafIndex` of root `rootCid`.
func GetShare(
	ctx context.Context,
	bGetter blockservice.BlockGetter,
	rootCid cid.Cid,
	leafIndex int,
	totalLeafs int, // this corresponds to the extended square width
) (Share, error) {
	nd, err := GetLeaf(ctx, bGetter, rootCid, leafIndex, totalLeafs)
	if err != nil {
		return nil, err
	}

	return leafToShare(nd), nil
}

// GetLeaf fetches and returns the raw leaf.
// It walks down the IPLD NMT tree until it finds the requested one.
func GetLeaf(ctx context.Context, bGetter blockservice.BlockGetter, root cid.Cid, leaf, total int) (ipld.Node, error) {
	// request the node
	nd, err := plugin.GetNode(ctx, bGetter, root)
	if err != nil {
		return nil, err
	}

	// look for links
	lnks := nd.Links()
	if len(lnks) == 1 {
		// in case there is only one we reached tree's bottom, so finally request the leaf.
		return plugin.GetNode(ctx, bGetter, lnks[0].Cid)
	}

	// route walk to appropriate children
	total /= 2 // as we are using binary tree, every step decreases total leaves in a half
	if leaf < total {
		root = lnks[0].Cid // if target leave on the left, go with walk down the first children
	} else {
		root, leaf = lnks[1].Cid, leaf-total // otherwise go down the second
	}

	// recursively walk down through selected children
	return GetLeaf(ctx, bGetter, root, leaf, total)
}

// GetProofsForShares fetches Merkle proofs for the given shares
// and returns the result as an array of ShareWithProof.
func GetProofsForShares(
	ctx context.Context,
	bGetter blockservice.BlockGetter,
	root cid.Cid,
	shares [][]byte,
) ([]*ShareWithProof, error) {
	proofs := make([]*ShareWithProof, len(shares))
	for index, share := range shares {
		if share != nil {
			proof := make([]cid.Cid, 0)
			// TODO(@vgonkivs): Combine GetLeafData and GetProof in one function as the are traversing the same tree.
			// Add options that will control what data will be fetched.
			s, err := GetLeaf(ctx, bGetter, root, index, len(shares))
			if err != nil {
				return nil, err
			}
			proof, err = GetProof(ctx, bGetter, root, proof, index, len(shares))
			if err != nil {
				return nil, err
			}
			proofs[index] = NewShareWithProof(index, s.RawData()[1:], proof)
		}
	}

	return proofs, nil
}

// GetProof fetches and returns the leaf's Merkle Proof.
// It walks down the IPLD NMT tree until it reaches the leaf and returns collected proof
func GetProof(
	ctx context.Context,
	bGetter blockservice.BlockGetter,
	root cid.Cid,
	proof []cid.Cid,
	leaf, total int,
) ([]cid.Cid, error) {
	// request the node
	nd, err := plugin.GetNode(ctx, bGetter, root)
	if err != nil {
		return nil, err
	}
	// look for links
	lnks := nd.Links()
	if len(lnks) == 1 {
		p := make([]cid.Cid, len(proof))
		copy(p, proof)
		return p, nil
	}

	// route walk to appropriate children
	total /= 2 // as we are using binary tree, every step decreases total leaves in a half
	if leaf < total {
		root = lnks[0].Cid // if target leave on the left, go with walk down the first children
		proof = append(proof, lnks[1].Cid)
	} else {
		root, leaf = lnks[1].Cid, leaf-total // otherwise go down the second
		proof, err = GetProof(ctx, bGetter, root, proof, leaf, total)
		if err != nil {
			return nil, err
		}
		return append(proof, lnks[0].Cid), nil
	}

	// recursively walk down through selected children
	return GetProof(ctx, bGetter, root, proof, leaf, total)
}

// GetSharesByNamespace returns all the shares from the given root
// with the given namespace.ID.
func GetSharesByNamespace(
	ctx context.Context,
	bGetter blockservice.BlockGetter,
	root cid.Cid,
	nID namespace.ID,
) ([]Share, error) {
	leaves := GetLeavesByNamespace(ctx, bGetter, root, nID)
	// if err != nil {
	// 	return nil, err
	// }

	shares := make([]Share, len(leaves))
	for i, leaf := range leaves {
		shares[i] = leafToShare(leaf)
	}

	return shares, nil
}

// GetLeavesByNamespace returns all the leaves from the given root with the given namespace.ID.
// If nothing is found it returns data as nil.
func GetLeavesByNamespace(
	ctx context.Context,
	bGetter blockservice.BlockGetter,
	root cid.Cid,
	nID namespace.ID,
) []ipld.Node {
	type job struct {
		id  cid.Cid
		pos int
	}

	// TODO: There has to be a more elegant solution
	// bookkeeping is needed to be able to sort the leaves after the walk
	type result struct {
		node ipld.Node
		pos  int
	}

	// TODO: Should this be NumWorkersLimit?
	// we don't know the amount of shares in the namespace, so we cannot preallocate properly
	jobs := make(chan *job, NumWorkersLimit)
	jobs <- &job{id: root}

	// the wg counter cannot be preallocated either, it is incremented with each job
	wg := sync.WaitGroup{}
	wg.Add(1)

	var leaves []result
	mu := &sync.Mutex{}

	// once the wait group is done, we can close the jobs channel to break the following loop
	go func() {
		wg.Wait()
		close(jobs)
	}()

	for j := range jobs {
		j := j
		// TODO: This is using the pool from get_shares.go, is this okay?
		pool.Submit(func() {
			defer wg.Done()
			err := SanityCheckNID(nID)
			if err != nil {
				return
			}

			rootH := plugin.NamespacedSha256FromCID(j.id)
			if nID.Less(nmt.MinNamespace(rootH, nID.Size())) || !nID.LessOrEqual(nmt.MaxNamespace(rootH, nID.Size())) {
				return
			}

			nd, err := plugin.GetNode(ctx, bGetter, j.id)
			if err != nil {
				return
			}

			lnks := nd.Links()
			// wg counter should be incremented before adding new jobs
			if len(lnks) > 1 {
				wg.Add(len(lnks))
			}

			if len(lnks) == 1 {
				mu.Lock()
				leaves = append(leaves, result{nd, j.pos})
				mu.Unlock()
				return
			}

			for i, lnk := range lnks {
				select {
				case jobs <- &job{
					id:  lnk.Cid,
					pos: j.pos*2 + i,
				}:
				case <-ctx.Done():
					return
				}
			}
		})
	}

	if len(leaves) > 0 {
		sort.SliceStable(leaves, func(i, j int) bool {
			return leaves[i].pos < leaves[j].pos
		})

		output := make([]ipld.Node, len(leaves))
		for i, leaf := range leaves {
			output[i] = leaf.node
		}
		return output
	}

	return nil
}

// leafToShare converts an NMT leaf into a Share.
func leafToShare(nd ipld.Node) Share {
	// * First byte represents the type of the node, and is unrelated to the actual share data
	// * Additional namespace is prepended so that parity data can be identified with a parity namespace, which we cut off
	return nd.RawData()[1+NamespaceSize:] // TODO(@Wondertan): Rework NMT/IPLD plugin to avoid the type byte
}
