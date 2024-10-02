package ipld

import (
	"context"
	"fmt"
	"sync"

	"github.com/ipfs/boxo/blockservice"
	"github.com/ipfs/boxo/ipld/merkledag"
	"github.com/ipfs/go-cid"
	ipld "github.com/ipfs/go-ipld-format"

	"github.com/celestiaorg/nmt"
)

type ctxKey int

const (
	proofsAdderKey ctxKey = iota
)

// NmtNodeAdder adds ipld.Nodes to the underlying ipld.Batch if it is inserted
// into a nmt tree.
type NmtNodeAdder struct {
	// lock protects Batch, Set and error from parallel writes / reads
	lock   sync.Mutex
	ctx    context.Context
	add    *ipld.Batch
	leaves *cid.Set
	err    error
}

// NewNmtNodeAdder returns a new NmtNodeAdder with the provided context and
// batch. Note that the context provided should have a timeout
// It is not thread-safe.
func NewNmtNodeAdder(ctx context.Context, bs blockservice.BlockService, opts ...ipld.BatchOption) *NmtNodeAdder {
	return &NmtNodeAdder{
		add:    ipld.NewBatch(ctx, merkledag.NewDAGService(bs), opts...),
		ctx:    ctx,
		leaves: cid.NewSet(),
	}
}

// Visit is a NodeVisitor that can be used during the creation of a new NMT to
// create and add ipld.Nodes to the Batch while computing the root of the NMT.
func (n *NmtNodeAdder) Visit(hash []byte, children ...[]byte) {
	n.lock.Lock()
	defer n.lock.Unlock()

	if n.err != nil {
		return // protect from further visits if there is an error
	}
	id := MustCidFromNamespacedSha256(hash)
	switch len(children) {
	case 1:
		if n.leaves.Visit(id) {
			n.err = n.add.Add(n.ctx, newNMTNode(id, children[0]))
		}
	case 2:
		n.err = n.add.Add(n.ctx, newNMTNode(id, append(children[0], children[1]...)))
	default:
		panic("expected a binary tree")
	}
}

// Commit checks for errors happened during Visit and if absent commits data to inner Batch.
func (n *NmtNodeAdder) Commit() error {
	n.lock.Lock()
	defer n.lock.Unlock()

	if n.err != nil {
		return fmt.Errorf("before batch commit: %w", n.err)
	}

	n.err = n.add.Commit()
	if n.err != nil {
		return fmt.Errorf("after batch commit: %w", n.err)
	}
	return nil
}

// MaxSizeBatchOption sets the maximum amount of buffered data before writing
// blocks.
func MaxSizeBatchOption(size int) ipld.BatchOption {
	return ipld.MaxSizeBatchOption(BatchSize(size))
}

// BatchSize calculates the amount of nodes that are generated from block of 'squareSizes'
// to be batched in one write.
func BatchSize(squareSize int) int {
	// (squareSize*2-1) - amount of nodes in a generated binary tree
	// squareSize*2 - the total number of trees, both for rows and cols
	// (squareSize*squareSize) - all the shares
	//
	// Note that while our IPLD tree looks like this:
	// ---X
	// -X---X
	// X-X-X-X
	// here we count leaves only once: the CIDs are the same for columns and rows
	// and for the last two layers as well:
	return (squareSize*2-1)*squareSize*2 - (squareSize * squareSize)
}

// ProofsAdder is used to collect proof nodes, while traversing merkle tree
type ProofsAdder struct {
	lock          sync.RWMutex
	collectShares bool
	proofs        map[cid.Cid][]byte
}

// NewProofsAdder creates new instance of ProofsAdder.
func NewProofsAdder(squareSize int, collectShares bool) *ProofsAdder {
	return &ProofsAdder{
		collectShares: collectShares,
		// preallocate map to fit all inner nodes for given square size
		proofs: make(map[cid.Cid][]byte, innerNodesAmount(squareSize)),
	}
}

// CtxWithProofsAdder creates context, that will contain ProofsAdder. If context is leaked to
// another go-routine, proofs will be not collected by gc. To prevent it, use Purge after Proofs
// are collected from adder, to preemptively release memory allocated for proofs.
func CtxWithProofsAdder(ctx context.Context, adder *ProofsAdder) context.Context {
	return context.WithValue(ctx, proofsAdderKey, adder)
}

// ProofsAdderFromCtx extracts ProofsAdder from context
func ProofsAdderFromCtx(ctx context.Context) *ProofsAdder {
	val := ctx.Value(proofsAdderKey)
	adder, ok := val.(*ProofsAdder)
	if !ok || adder == nil {
		return nil
	}
	return adder
}

// Proofs returns proofs collected by ProofsAdder
func (a *ProofsAdder) Proofs() map[cid.Cid][]byte {
	if a == nil {
		return nil
	}

	a.lock.RLock()
	defer a.lock.RUnlock()
	return a.proofs
}

// VisitFn returns NodeVisitorFn, that will collect proof nodes while traversing merkle tree.
func (a *ProofsAdder) VisitFn() nmt.NodeVisitorFn {
	if a == nil {
		return nil
	}

	a.lock.RLock()
	defer a.lock.RUnlock()

	// proofs are already collected, don't collect second time
	if len(a.proofs) > 0 {
		return nil
	}
	return a.visitNodes
}

// Purge removed proofs from ProofsAdder allowing GC to collect the memory
func (a *ProofsAdder) Purge() {
	if a == nil {
		return
	}

	a.lock.Lock()
	defer a.lock.Unlock()

	a.proofs = nil
}

func (a *ProofsAdder) visitNodes(hash []byte, children ...[]byte) {
	switch len(children) {
	case 1:
		if a.collectShares {
			id := MustCidFromNamespacedSha256(hash)
			a.addProof(id, children[0])
		}
	case 2:
		id := MustCidFromNamespacedSha256(hash)
		a.addProof(id, append(children[0], children[1]...))
	default:
		panic("expected a binary tree")
	}
}

func (a *ProofsAdder) addProof(id cid.Cid, proof []byte) {
	a.lock.Lock()
	defer a.lock.Unlock()
	a.proofs[id] = proof
}

// innerNodesAmount return amount of inner nodes in eds with given size
func innerNodesAmount(squareSize int) int {
	return 2 * (squareSize - 1) * squareSize
}
