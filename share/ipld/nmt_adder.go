package ipld

import (
	"context"
	"fmt"
	"sync"

	"github.com/ipfs/go-blockservice"
	"github.com/ipfs/go-cid"
	ipld "github.com/ipfs/go-ipld-format"
	"github.com/ipfs/go-libipfs/blocks"
	"github.com/ipfs/go-merkledag"

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

type ProofsAdder struct {
	lock   sync.Mutex
	proofs []blocks.Block
}

func NewProofsAdder(squareSize int) *ProofsAdder {
	return &ProofsAdder{
		// preallocate array to fit all inner nodes for given square size
		proofs: make([]blocks.Block, 0, 2*(squareSize-1)*squareSize),
	}
}

func CtxWithProofsAdder(ctx context.Context, adder *ProofsAdder) context.Context {
	return context.WithValue(ctx, proofsAdderKey, adder)
}

func ProofsAdderFromCtx(ctx context.Context) *ProofsAdder {
	val := ctx.Value(proofsAdderKey)
	adder, ok := val.(*ProofsAdder)
	if !ok || adder == nil {
		return nil
	}
	return adder
}

func (a *ProofsAdder) Proofs() []blocks.Block {
	if a == nil {
		return nil
	}

	a.lock.Lock()
	defer a.lock.Unlock()
	return a.proofs
}

func (a *ProofsAdder) VisitFn() nmt.NodeVisitorFn {
	if a == nil {
		return nil
	}

	// proofs are already collected, don't collect second time
	if len(a.proofs) > 0 {
		return nil
	}
	return a.visitInnerNodes
}

func (a *ProofsAdder) visitInnerNodes(hash []byte, children ...[]byte) {
	switch len(children) {
	case 1:
		break
	case 2:
		id := MustCidFromNamespacedSha256(hash)
		b, err := blocks.NewBlockWithCid(append(children[0], children[1]...), id)
		if err != nil {
			panic(err)
		}
		a.lock.Lock()
		defer a.lock.Unlock()
		a.proofs = append(a.proofs, b)
	default:
		panic("expected a binary tree")
	}
}
