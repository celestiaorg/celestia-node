package ipld

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/gammazero/workerpool"
	"github.com/ipfs/go-blockservice"
	"github.com/ipfs/go-cid"
	ipld "github.com/ipfs/go-ipld-format"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"

	"github.com/celestiaorg/nmt"
	"github.com/celestiaorg/nmt/namespace"
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
var NumWorkersLimit = MaxSquareSize * MaxSquareSize / 2 * NumConcurrentSquares

// NumConcurrentSquares limits the amount of squares that are fetched
// concurrently/simultaneously.
var NumConcurrentSquares = 8

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
	if len(lnks) == 1 {
		// in case there is only one we reached tree's bottom, so finally request the leaf.
		return GetNode(ctx, bGetter, lnks[0].Cid)
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
	ctx, span := tracer.Start(ctx, "get-leaves")
	defer span.End()

	// this buffer ensures writes to 'jobs' are never blocking (bin-tree-feat)
	jobs := make(chan *job, (maxShares+1)/2) // +1 for the case where 'maxShares' is 1
	jobs <- &job{id: root, ctx: ctx}
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
				ctx, span := tracer.Start(j.ctx, "process-job")
				defer span.End()
				defer wg.Done()

				span.SetAttributes(
					attribute.String("cid", j.id.String()),
					attribute.Int("pos", j.pos),
				)

				nd, err := GetNode(ctx, bGetter, j.id)
				if err != nil {
					// we don't really care about errors here
					// just fetch as much as possible
					span.RecordError(err)
					span.SetStatus(codes.Error, err.Error())
					return
				}
				// check links to know what we should do with the node
				lnks := nd.Links()
				if len(lnks) == 1 { // so we are almost there
					// the reason why the comment on 'total' is lying, as each
					// leaf has its own additional leaf(hack) so get it
					nd, err := GetNode(ctx, bGetter, lnks[0].Cid)
					if err != nil {
						// again, we don't really care much, just fetch as much as possible
						span.RecordError(err)
						span.SetStatus(codes.Error, err.Error())
						return
					}
					// successfully fetched a share/leaf
					// ladies and gentlemen, we got em!
					span.SetStatus(codes.Ok, "")
					put(j.pos, nd)
					return
				}
				// ok, we found more links
				for i, lnk := range lnks {
					// send those to be processed
					select {
					case jobs <- &job{
						id: lnk.Cid,
						// calc position for children nodes (bin-tree-feat),
						// s.t. 'if' above knows where to put a share
						pos: j.pos*2 + i,
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

// GetLeavesByNamespace returns as many leaves from the given root with the given namespace.ID as it can retrieve.
// If no shares are found, it returns both data and error as nil.
// A non-nil error means that only partial data is returned, because at least one share retrieval failed
// The following implementation is based on `GetShares`.
func GetLeavesByNamespace(
	ctx context.Context,
	bGetter blockservice.BlockGetter,
	root cid.Cid,
	nID namespace.ID,
	maxShares int,
) ([]ipld.Node, error) {
	if len(nID) != NamespaceSize {
		return nil, fmt.Errorf("expected namespace ID of size %d, got %d", NamespaceSize, len(nID))
	}

	ctx, span := tracer.Start(ctx, "get-leaves-by-namespace")
	defer span.End()

	span.SetAttributes(
		attribute.String("namespace", nID.String()),
		attribute.String("root", root.String()),
	)

	// we don't know where in the tree the leaves in the namespace are,
	// so we keep track of the bounds to return the correct slice
	// maxShares acts as a sentinel to know if we find any leaves
	bounds := fetchedBounds{int64(maxShares), 0}

	// buffer the jobs to avoid blocking, we only need as many
	// queued as the number of shares in the second-to-last layer
	jobs := make(chan *job, (maxShares+1)/2)
	jobs <- &job{id: root, ctx: ctx}

	var wg wrappedWaitGroup
	wg.jobs = jobs
	wg.add(1)

	var (
		singleErr    sync.Once
		retrievalErr error
	)

	// we overallocate space for leaves since we do not know how many we will find
	// on the level above, the length of the Row is passed in as maxShares
	leaves := make([]ipld.Node, maxShares)

	for {
		select {
		case j, ok := <-jobs:
			if !ok {
				// if there were no leaves under the given root in the given namespace,
				// both return values are nil. otherwise, the error will also be non-nil.
				if bounds.lowest == int64(maxShares) {
					return nil, retrievalErr
				}

				return leaves[bounds.lowest : bounds.highest+1], retrievalErr
			}
			pool.Submit(func() {
				ctx, span := tracer.Start(j.ctx, "process-job")
				defer span.End()
				defer wg.done()

				span.SetAttributes(
					attribute.String("cid", j.id.String()),
					attribute.Int("pos", j.pos),
				)

				// if an error is likely to be returned or not depends on
				// the underlying impl of the blockservice, currently it is not a realistic probability
				nd, err := GetNode(ctx, bGetter, j.id)
				if err != nil {
					singleErr.Do(func() {
						retrievalErr = err
					})
					log.Errorw("getSharesByNamespace: could not retrieve node", "nID", nID, "pos", j.pos, "err", err)
					span.SetStatus(codes.Error, err.Error())
					// we still need to update the bounds
					bounds.update(int64(j.pos))
					return
				}

				links := nd.Links()
				linkCount := uint64(len(links))
				if linkCount == 1 {
					// successfully fetched a leaf belonging to the namespace
					span.SetStatus(codes.Ok, "")
					leaves[j.pos] = nd
					// we found a leaf, so we update the bounds
					// the update routine is repeated until the atomic swap is successful
					bounds.update(int64(j.pos))
					return
				}

				// this node has links in the namespace, so keep walking
				for i, lnk := range links {
					newJob := &job{
						id: lnk.Cid,
						// position represents the index in a flattened binary tree,
						// so we can return a slice of leaves in order
						pos: j.pos*2 + i,
						// we pass the context to job so that spans are tracked in a tree
						// structure
						ctx: ctx,
					}

					// if the link's nID isn't in range we don't need to create a new job for it
					jobNid := NamespacedSha256FromCID(newJob.id)
					if nID.Less(nmt.MinNamespace(jobNid, nID.Size())) || !nID.LessOrEqual(nmt.MaxNamespace(jobNid, nID.Size())) {
						continue
					}

					// by passing the previous check, we know we will have one more node to process
					// note: it is important to increase the counter before sending to the channel
					wg.add(1)
					select {
					case jobs <- newJob:
					case <-ctx.Done():
						return
					}
				}
			})
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

// wrappedWaitGroup is needed because waitgroups do not expose their internal counter,
// and we don't know in advance how many jobs we will have to wait for.
type wrappedWaitGroup struct {
	wg      sync.WaitGroup
	jobs    chan *job
	counter int64
}

func (w *wrappedWaitGroup) add(count int64) {
	w.wg.Add(int(count))
	atomic.AddInt64(&w.counter, count)
}

func (w *wrappedWaitGroup) done() {
	w.wg.Done()
	numRemaining := atomic.AddInt64(&w.counter, -1)

	// Close channel if this job was the last one
	if numRemaining == 0 {
		close(w.jobs)
	}
}

type fetchedBounds struct {
	lowest  int64
	highest int64
}

// update checks if the passed index is outside the current bounds,
// and updates the bounds atomically if it extends them.
func (b *fetchedBounds) update(index int64) {
	lowest := atomic.LoadInt64(&b.lowest)
	// try to write index to the lower bound if appropriate, and retry until the atomic op is successful
	// CAS ensures that we don't overwrite if the bound has been updated in another goroutine after the comparison here
	for index < lowest && !atomic.CompareAndSwapInt64(&b.lowest, lowest, index) {
		lowest = atomic.LoadInt64(&b.lowest)
	}
	// we always run both checks because element can be both the lower and higher bound
	// for example, if there is only one share in the namespace
	highest := atomic.LoadInt64(&b.highest)
	for index > highest && !atomic.CompareAndSwapInt64(&b.highest, highest, index) {
		highest = atomic.LoadInt64(&b.highest)
	}
}

// job represents an encountered node to investigate during the `GetLeaves`
// and `GetLeavesByNamespace` routines.
type job struct {
	id  cid.Cid
	pos int
	ctx context.Context
}
