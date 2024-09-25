package ipld

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"

	"github.com/gammazero/workerpool"
	"github.com/ipfs/boxo/blockservice"
	"github.com/ipfs/go-cid"
	ipld "github.com/ipfs/go-ipld-format"

	"github.com/celestiaorg/celestia-node/share"
)

// NumWorkersLimit sets global limit for workers spawned by GetShares.
// GetShares could be called MaxSquareSize(128) times per data square each
// spawning up to 128/2 goroutines and altogether this is 8192. Considering
// there can be N blocks fetched at the same time, e.g. during catching up data
// from the past, we multiply this number by the amount of allowed concurrent
// data square fetches(NumConcurrentSquares).
//
// NOTE: This value only limits amount of simultaneously running workers that
// are spawned as the load increases and are killed, once the load declines.
//
// TODO(@Wondertan): This assumes we have parallelized DASer implemented. Sync the values once it is shipped.
// TODO(@Wondertan): Allow configuration of values without global state.
var NumWorkersLimit = share.MaxSquareSize * share.MaxSquareSize / 2 * NumConcurrentSquares

// NumConcurrentSquares limits the amount of squares that are fetched
// concurrently/simultaneously.
var NumConcurrentSquares = 8

// ErrNodeNotFound is used to signal when a nmt Node could not be found.
var ErrNodeNotFound = errors.New("nmt node not found")

// Global worker pool that globally controls and limits goroutines spawned by
// GetShares.
//
//	TODO(@Wondertan): Idle timeout for workers needs to be configured to around block time,
//		so that workers spawned between each reconstruction for every new block are reused.
var pool = workerpool.New(NumWorkersLimit)

// GetLeaf fetches and returns the raw leaf.
// It walks down the IPLD NMT tree until it finds the requested one.
func GetLeaf(
	ctx context.Context,
	bGetter blockservice.BlockGetter,
	root cid.Cid,
	leaf, total int,
) (ipld.Node, error) {
	// request the node
	nd, err := GetNode(ctx, bGetter, root)
	if err != nil {
		return nil, err
	}

	// look for links
	lnks := nd.Links()
	if len(lnks) == 0 {
		// in case there is none, we reached tree's bottom, so finally return the leaf.
		return nd, err
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

// GetLeaves gets leaves from either local storage, or, if not found, requests
// them from immediate/connected peers. It puts them into the slice under index
// of node position in the tree (bin-tree-feat).
// Does not return any error, and returns/unblocks only on success
// (got all shares) or on context cancellation.
//
// It works concurrently by spawning workers in the pool which do one basic
// thing - block until data is fetched, s. t. share processing is never
// sequential, and thus we request *all* the shares available without waiting
// for others to finish. It is the required property to maximize data
// availability. As a side effect, we get concurrent tree traversal reducing
// time to data time.
//
// GetLeaves relies on the fact that the underlying data structure is a binary
// tree, so it's not suitable for anything else besides that. Parts on the
// implementation that rely on this property are explicitly tagged with
// (bin-tree-feat).
func GetLeaves(ctx context.Context,
	bGetter blockservice.BlockGetter,
	root cid.Cid,
	maxShares int,
	put func(int, ipld.Node),
) {
	// this buffer ensures writes to 'jobs' are never blocking (bin-tree-feat)
	jobs := make(chan *job, (maxShares+1)/2) // +1 for the case where 'maxShares' is 1
	jobs <- &job{cid: root, ctx: ctx}
	// total is an amount of routines spawned and total amount of nodes we process (bin-tree-feat)
	// so we can specify exact amount of loops we do, and wait for this amount
	// of routines to finish processing
	total := maxShares*2 - 1
	wg := sync.WaitGroup{}
	wg.Add(total)

	// all preparations are done, so begin processing jobs
	for i := 0; i < total; i++ {
		select {
		case j := <-jobs:
			// work over each job concurrently, s.t. shares do not block
			// processing of each other
			pool.Submit(func() {
				defer wg.Done()

				nd, err := GetNode(ctx, bGetter, j.cid)
				if err != nil {
					// we don't really care about errors here
					// just fetch as much as possible
					return
				}
				// check links to know what we should do with the node
				lnks := nd.Links()
				if len(lnks) == 0 {
					// successfully fetched a share/leaf
					// ladies and gentlemen, we got em!
					put(j.sharePos, nd)
					return
				}
				// ok, we found more links
				for i, lnk := range lnks {
					// send those to be processed
					select {
					case jobs <- &job{
						cid: lnk.Cid,
						// calc position for children nodes (bin-tree-feat),
						// s.t. 'if' above knows where to put a share
						sharePos: j.sharePos*2 + i,
						// we pass the context to job so that spans are tracked in a tree
						// structure
						ctx: ctx,
					}:
					case <-ctx.Done():
						return
					}
				}
			})
		case <-ctx.Done():
			return
		}
	}
	// "tick-tack, how much more should I wait before you get those shares?" - the goroutine
	wg.Wait()
}

// chanGroup implements an atomic wait group, closing a jobs chan
// when fully done.
type chanGroup struct {
	jobs    chan job
	counter atomic.Int64
}

func (w *chanGroup) add(count int64) {
	w.counter.Add(count)
}

func (w *chanGroup) done() {
	numRemaining := w.counter.Add(-1)

	// Close channel if this job was the last one
	if numRemaining == 0 {
		close(w.jobs)
	}
}

// job represents an encountered node to investigate during the `GetLeaves`
// and `CollectLeavesByNamespace` routines.
type job struct {
	// we pass the context to job so that spans are tracked in a tree
	// structure
	ctx context.Context
	// cid of the node that will be handled
	cid cid.Cid
	// sharePos represents potential share position in share slice
	sharePos int
	// depth represents the number of edges present in path from the root node of a tree to that node
	depth int
	// isAbsent indicates if target namespaceID is not included, only collect absence proofs
	isAbsent bool
}

func (j job) next(direction direction, cid cid.Cid, isAbsent bool) job {
	var i int
	if direction == right {
		i++
	}
	return job{
		ctx:      j.ctx,
		cid:      cid,
		sharePos: j.sharePos*2 + i,
		depth:    j.depth + 1,
		isAbsent: isAbsent,
	}
}
