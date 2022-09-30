package share

import (
	"context"
	"sync"
	"sync/atomic"

	format "github.com/ipfs/go-ipld-format"

	"github.com/celestiaorg/celestia-node/share/ipld"

	"github.com/gammazero/workerpool"
	"github.com/ipfs/go-blockservice"
	"github.com/ipfs/go-cid"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"

	"github.com/celestiaorg/nmt"
	"github.com/celestiaorg/nmt/namespace"
)

// TODO(@distractedm1nd) Find a better figure than NumWorkersLimit for this pool. Issue #970
// namespacePool is a worker pool responsible for the goroutines spawned by getLeavesByNamespace
var namespacePool = workerpool.New(NumWorkersLimit)

// GetSharesByNamespace walks the tree of a given root and returns its shares within the given namespace.ID.
// If a share could not be retrieved, err is not nil, and the returned array
// contains nil shares in place of the shares it was unable to retrieve.
func GetSharesByNamespace(
	ctx context.Context,
	bGetter blockservice.BlockGetter,
	root cid.Cid,
	nID namespace.ID,
	maxShares int,
) ([]Share, error) {
	ctx, span := tracer.Start(ctx, "get-shares-by-namespace")
	defer span.End()

	leaves, err := getLeavesByNamespace(ctx, bGetter, root, nID, maxShares)
	if err != nil && leaves == nil {
		return nil, err
	}

	shares := make([]Share, len(leaves))
	for i, leaf := range leaves {
		if leaf != nil {
			shares[i] = leafToShare(leaf)
		}
	}

	return shares, err
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

// getLeavesByNamespace returns as many leaves from the given root with the given namespace.ID as it can retrieve.
// If no shares are found, it returns both data and error as nil.
// A non-nil error means that only partial data is returned, because at least one share retrieval failed
// The following implementation is based on `GetShares`.
func getLeavesByNamespace(
	ctx context.Context,
	bGetter blockservice.BlockGetter,
	root cid.Cid,
	nID namespace.ID,
	maxShares int,
) ([]format.Node, error) {
	err := SanityCheckNID(nID)
	if err != nil {
		return nil, err
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
	leaves := make([]format.Node, maxShares)

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
			namespacePool.Submit(func() {
				ctx, span := tracer.Start(j.ctx, "process-job")
				defer span.End()
				defer wg.done()

				span.SetAttributes(
					attribute.String("cid", j.id.String()),
					attribute.Int("pos", j.pos),
				)

				// if an error is likely to be returned or not depends on
				// the underlying impl of the blockservice, currently it is not a realistic probability
				nd, err := ipld.GetNode(ctx, bGetter, j.id)
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
					jobNid := ipld.NamespacedSha256FromCID(newJob.id)
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
