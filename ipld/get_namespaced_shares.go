package ipld

import (
	"context"
	"sort"
	"sync"

	"github.com/celestiaorg/celestia-node/ipld/plugin"
	"github.com/celestiaorg/nmt"
	"github.com/celestiaorg/nmt/namespace"
	"github.com/gammazero/workerpool"
	"github.com/ipfs/go-blockservice"
	"github.com/ipfs/go-cid"
	ipld "github.com/ipfs/go-ipld-format"
)

// TODO(@distractedm1nd) Should the namespace pool use NumWorkersLimit? This should probably be configurable.
// Worker pool responsible for the goroutines spawned by GetLeavesByNamespace
var namespacePool = workerpool.New(NumWorkersLimit)

// GetSharesByNamespace returns all the shares from the given root
// with the given namespace.ID.
func GetSharesByNamespace(
	ctx context.Context,
	bGetter blockservice.BlockGetter,
	root cid.Cid,
	nID namespace.ID,
) ([]Share, error) {
	leaves := GetLeavesByNamespace(ctx, bGetter, root, nID)

	shares := make([]Share, len(leaves))
	for i, leaf := range leaves {
		shares[i] = leafToShare(leaf)
	}

	return shares, nil
}

type WrappedWaitGroup struct {
	wg      sync.WaitGroup
	mu      sync.Mutex
	counter int
}

func (w *WrappedWaitGroup) Add(count int) {
	w.wg.Add(count)
	w.mu.Lock()
	w.counter += count
	w.mu.Unlock()
}

func (w *WrappedWaitGroup) Done() {
	w.wg.Done()
	w.mu.Lock()
	w.counter--
	w.mu.Unlock()
}

func (w *WrappedWaitGroup) Wait() {
	w.wg.Wait()
}

func (w *WrappedWaitGroup) IsWorking() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.counter > 0
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
	wg := WrappedWaitGroup{sync.WaitGroup{}, sync.Mutex{}, 0}
	wg.Add(1)

	var leaves []result
	mu := &sync.Mutex{}

	for wg.IsWorking() {
		select {
		case j := <-jobs:
			namespacePool.Submit(func() {
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
				linkCount := len(lnks)
				// wg counter should be incremented before adding new jobs
				if linkCount > 1 {
					wg.Add(linkCount)
				} else if linkCount == 1 {
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
		case <-ctx.Done():
			return nil
		default:
		}
	}

	wg.Wait()
	if len(leaves) > 0 {
		sort.Slice(leaves, func(i, j int) bool {
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
