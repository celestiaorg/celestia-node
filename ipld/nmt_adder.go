package ipld

import (
	"context"
	"github.com/celestiaorg/celestia-node/edsstore"
	"github.com/celestiaorg/celestia-node/ipld/plugin"
	"github.com/filecoin-project/dagstore"
	"github.com/filecoin-project/dagstore/mount"
	"github.com/filecoin-project/dagstore/shard"
	"github.com/ipfs/go-blockservice"
	"github.com/ipfs/go-cid"
	ipld "github.com/ipfs/go-ipld-format"
	"github.com/ipfs/go-merkledag"
	"github.com/ipld/go-car/v2/blockstore"
	"os"
)

// NmtNodeAdder adds ipld.Nodes to the underlying ipld.Batch if it is inserted
// into a nmt tree.
type NmtNodeAdder struct {
	ctx    context.Context
	bs     *edsstore.EDSStore
	key    string
	roots  []cid.Cid
	rw     *blockstore.ReadWrite
	add    *ipld.Batch
	leaves *cid.Set
	err    error
}

// NewNmtNodeAdder returns a new NmtNodeAdder with the provided context and
// batch. Note that the context provided should have a timeout
// It is not thread-safe.
func NewNmtNodeAdder(ctx context.Context, roots []cid.Cid, key string, bs blockservice.BlockService, edsStr *edsstore.EDSStore, opts ...ipld.BatchOption) *NmtNodeAdder {
	carBlockStore, err := blockstore.OpenReadWrite("/tmp/carexample/"+key, roots, blockstore.AllowDuplicatePuts(true))
	if err != nil {
		panic(err)
	}
	return &NmtNodeAdder{
		add:    ipld.NewBatch(ctx, edsstore.New(carBlockStore, bs.Exchange()), opts...),
		bs:     edsStr,
		key:    key,
		roots:  roots,
		rw:     carBlockStore,
		ctx:    ctx,
		leaves: cid.NewSet(),
	}
}

func NewBasicNmtNodeAdder(ctx context.Context, bs blockservice.BlockService, opts ...ipld.BatchOption) *NmtNodeAdder {
	return &NmtNodeAdder{
		add:    ipld.NewBatch(ctx, merkledag.NewDAGService(bs), opts...),
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
			n.err = n.add.Add(n.ctx, plugin.NewNMTLeafNode(id, children[0]))
		}
	case 2:
		n.err = n.add.Add(n.ctx, plugin.NewNMTNode(id, children[0], children[1]))
	default:
		panic("expected a binary tree")
	}
}

// Commit checks for errors happened during Visit and if absent commits data to inner Batch.
func (n *NmtNodeAdder) Commit() error {
	if n.err != nil {
		return n.err
	}

	err := n.add.Commit()
	if err != nil {
		log.Errorw("error committing batch", "key", n.key, "err", err)
	}

	if n.bs != nil {
		err = n.rw.Finalize()
		if err != nil {
			log.Errorw("couldn't finalize", "key", n.key, "err", err)
			return err
		}
		// TODO: context
		err = n.bs.RegisterShard(context.Background(), shard.KeyFromString(n.key), &mount.FSMount{
			FS:   os.DirFS("/tmp/carexample/"),
			Path: n.key,
		}, nil, dagstore.RegisterOpts{})
		if err != nil {
			log.Warnw("couldn't register shard", "key", n.key, "err", err)
		}
		return nil
	} else {
		return err
	}
}
