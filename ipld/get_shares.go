package ipld

import (
	"context"
	"sync"

	"github.com/gammazero/workerpool"
	"github.com/ipfs/go-blockservice"
	"github.com/ipfs/go-cid"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"

	"github.com/celestiaorg/celestia-node/ipld/plugin"
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
// TODO(@Wondertan): This assumes we have parallelized DASer implemented.
//  Sync the values once it is shipped.
// TODO(@Wondertan): Allow configuration of values without global state.
var NumWorkersLimit = MaxSquareSize * MaxSquareSize / 2 * NumConcurrentSquares

// NumConcurrentSquares limits the amount of squares that are fetched
// concurrently/simultaneously.
var NumConcurrentSquares = 8

// Global worker pool that globally controls and limits goroutines spawned by
// GetShares.
// TODO(@Wondertan): Idle timeout for workers needs to be configured to around block time,
// 	so that workers spawned between each reconstruction for every new block are reused.
var pool = workerpool.New(NumWorkersLimit)

// GetShares gets shares from either local storage, or, if not found, requests
// them from immediate/connected peers. It puts them into the given 'put' func,
// does not return any error, and returns/unblocks only on success
// (got all shares) or on context cancellation.
//
// It works concurrently by spawning workers in the pool which do one basic
// thing - block until data is fetched, s. t. share processing is never
// sequential, and thus we request *all* the shares available without waiting
// for others to finish. It is the required property to maximize data
// availability. As a side effect, we get concurrent tree traversal reducing
// time to data time.
//
// GetShares relies on the fact that the underlying data structure is a binary
// tree, so it's not suitable for anything else besides that. Parts on the
// implementation that rely on this property are explicitly tagged with
// (bin-tree-feat).
func GetShares(ctx context.Context, bGetter blockservice.BlockGetter, root cid.Cid, shares int, put func(int, Share)) {
	ctx, span := tracer.Start(ctx, "get-shares")
	defer span.End()

	// this buffer ensures writes to 'jobs' are never blocking (bin-tree-feat)
	jobs := make(chan *job, (shares+1)/2) // +1 for the case where 'shares' is 1
	jobs <- &job{id: root, ctx: ctx}
	// total is an amount of routines spawned and total amount of nodes we process (bin-tree-feat)
	// so we can specify exact amount of loops we do, and wait for this amount
	// of routines to finish processing
	total := shares*2 - 1
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

				nd, err := plugin.GetNode(ctx, bGetter, j.id)
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
					nd, err := plugin.GetNode(ctx, bGetter, lnks[0].Cid)
					if err != nil {
						// again, we don't really care much, just fetch as much as possible
						span.RecordError(err)
						span.SetStatus(codes.Error, err.Error())
						return
					}
					// successfully fetched a share/leaf
					// ladies and gentlemen, we got em!
					span.SetStatus(codes.Ok, "")
					put(j.pos, leafToShare(nd))
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
