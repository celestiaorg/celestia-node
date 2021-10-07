package ipld

import (
	"context"

	"github.com/ipfs/go-cid"
	ipld "github.com/ipfs/go-ipld-format"

	"github.com/celestiaorg/celestia-node/ipld/plugin"
)

// NmtNodeAdder adds ipld.Nodes to the underlying ipld.Batch if it is inserted
// into an nmt tree
type NmtNodeAdder struct {
	ctx    context.Context
	batch  *ipld.Batch
	leaves *cid.Set
	err    error
}

// NewNmtNodeAdder returns a new NmtNodeAdder with the provided context and
// batch. Note that the context provided should have a timeout
// It is not thread-safe.
func NewNmtNodeAdder(ctx context.Context, batch *ipld.Batch) *NmtNodeAdder {
	return &NmtNodeAdder{
		batch:  batch,
		ctx:    ctx,
		leaves: cid.NewSet(),
	}
}

// Visit is a NodeVisitor that can be used during the creation of a new NMT to
// create and add ipld.Nodes to the Batch while computing the root of the NMT.
func (n *NmtNodeAdder) Visit(hash []byte, children ...[]byte) {
	if n.err != nil {
		return // protect from further visits if there is an error
	}

	id := plugin.MustCidFromNamespacedSha256(hash)
	switch len(children) {
	case 1:
		if n.leaves.Visit(id) {
			n.err = n.batch.Add(n.ctx, plugin.NewNMTLeafNode(id, children[0]))
		}
	case 2:
		n.err = n.batch.Add(n.ctx, plugin.NewNMTNode(id, children[0], children[1]))
	default:
		panic("expected a binary tree")
	}
}

// Batch return the ipld.Batch originally provided to the NmtNodeAdder
func (n *NmtNodeAdder) Batch() *ipld.Batch {
	return n.batch
}

// Commit checks for errors happened during Visit and if absent commits data to inner Batch.
func (n *NmtNodeAdder) Commit() error {
	if n.err != nil {
		return n.err
	}

	return n.batch.Commit()
}
