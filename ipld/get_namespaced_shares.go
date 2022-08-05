package ipld

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/gammazero/workerpool"
	"github.com/ipfs/go-blockservice"
	"github.com/ipfs/go-cid"
	ipld "github.com/ipfs/go-ipld-format"

	"github.com/celestiaorg/celestia-node/ipld/plugin"
	"github.com/celestiaorg/nmt"
	"github.com/celestiaorg/nmt/namespace"
)

// TODO(@distractedm1nd) Find a better figure than NumWorkersLimit for this pool.
// Worker pool responsible for the goroutines spawned by getLeavesByNamespace
var namespacePool = workerpool.New(NumWorkersLimit)

// GetSharesByNamespace walks the tree of a given root and returns its shares within the given namespace.ID.
func GetSharesByNamespace(
	ctx context.Context,
	bGetter blockservice.BlockGetter,
	root cid.Cid,
	nID namespace.ID,
	maxShares int,
) ([]Share, error) {
	leaves, err := getLeavesByNamespace(ctx, bGetter, root, nID, maxShares)
	if err != nil {
		return nil, err
	}

	shares := make([]Share, len(leaves))
	for i, leaf := range leaves {
		shares[i] = leafToShare(leaf)
	}

	return shares, nil
}

// wrappedWaitGroup is needed because waitgroups do not expose their internal counter,
// and we don't know in advance how many jobs we will have to wait for.
type wrappedWaitGroup struct {
	wg      sync.WaitGroup
	jobs    chan *job
	counter int64
}

func (w *wrappedWaitGroup) Add(count int64) {
	w.wg.Add(int(count))
	atomic.AddInt64(&w.counter, count)
}

func (w *wrappedWaitGroup) Done() {
	w.wg.Done()
	atomic.AddInt64(&w.counter, -1)
	if atomic.LoadInt64(&w.counter) == 0 {
		close(w.jobs)
	}
}

func (w *wrappedWaitGroup) Wait() {
	w.wg.Wait()
}

type fetchedBounds struct {
	lowest  int64
	highest int64
}

func (b *fetchedBounds) Update(index int64) {
	for {
		lowest := atomic.LoadInt64(&b.lowest)
		if index >= lowest || atomic.CompareAndSwapInt64(&b.lowest, lowest, index) {
			break
		}
	}

	for {
		highest := atomic.LoadInt64(&b.highest)
		if index <= highest || atomic.CompareAndSwapInt64(&b.highest, highest, index) {
			break
		}
	}
}

// getLeavesByNamespace returns all the leaves from the given root with the given namespace.ID.
// If nothing is found, it returns both data and error as nil.
// The following implementation is based on `GetShares`.
func getLeavesByNamespace(
	ctx context.Context,
	bGetter blockservice.BlockGetter,
	root cid.Cid,
	nID namespace.ID,
	maxShares int,
) ([]ipld.Node, error) {
	err := SanityCheckNID(nID)
	if err != nil {
		return nil, err
	}

	// we don't know where in the tree the leaves in the namespace are,
	// so we keep track of the bounds to return the correct slice
	// maxShares acts as a sentinel to know if we find any leaves
	bounds := fetchedBounds{int64(maxShares), 0}

	// buffer the jobs to avoid blocking, we only need as many
	// queued as the number of shares in the second-to-last layer
	jobs := make(chan *job, (maxShares+1)/2)
	jobs <- &job{id: root}

	var wg wrappedWaitGroup
	wg.jobs = jobs
	wg.Add(1)

	// we overallocate space for leaves since we do not know how many we will find
	// on the level above, the length of the Row is passed in as maxShares
	leaves := make([]ipld.Node, maxShares)

	// stillWorking will be set to false when the waitgroup counter reaches 0, closing the jobs channel
	for {
		select {
		case j, ok := <-jobs:
			if !ok {
				// we didn't find any leaves, return nil
				if bounds.lowest == int64(maxShares) {
					return nil, nil
				}

				return leaves[bounds.lowest : bounds.highest+1], nil
			}
			namespacePool.Submit(func() {
				defer wg.Done()

				rootH := plugin.NamespacedSha256FromCID(j.id)
				if nID.Less(nmt.MinNamespace(rootH, nID.Size())) || !nID.LessOrEqual(nmt.MaxNamespace(rootH, nID.Size())) {
					return
				}

				nd, err := plugin.GetNode(ctx, bGetter, j.id)
				if err != nil {
					return
				}

				links := nd.Links()
				linkCount := uint64(len(links))
				// wg counter needs to be incremented **before** adding new jobs
				if linkCount > 1 {
					wg.Add(int64(linkCount))
				} else if linkCount == 1 {
					// successfully fetched a leaf belonging to the namespace
					leaves[j.pos] = nd
					// we found a leaf, so we update the bounds
					// the update routine is repeated until the atomic swap is successful
					bounds.Update(int64(j.pos))
					return
				}

				// this node has links in the namespace, so keep walking
				for i, lnk := range links {
					select {
					case jobs <- &job{
						id: lnk.Cid,
						// position represents the index in a flattened binary tree,
						// so we can return a slice of leaves in order
						pos: j.pos*2 + i,
					}:
					case <-ctx.Done():
						return
					}
				}
			})
		case <-ctx.Done():
			return nil, nil
		}
	}
}
