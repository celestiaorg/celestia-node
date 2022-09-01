package dagblockstore

import (
	"context"
	"errors"
	"fmt"
	"github.com/filecoin-project/dagstore"
	"github.com/filecoin-project/dagstore/index"
	"github.com/filecoin-project/dagstore/mount"
	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/namespace"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	ipld "github.com/ipfs/go-ipld-format"
	"os"
)

var (
	_                   blockstore.Blockstore = (*DAGBlockStore)(nil)
	invertedIndexPrefix                       = datastore.NewKey("/index/inverted")
)

type DAGBlockStore struct {
	dagstore.DAGStore
}

func NewDAGBlockStore(ds datastore.Batching) *DAGBlockStore {
	r := mount.NewRegistry()
	err := r.Register("fs", &mount.FSMount{FS: os.DirFS("/tmp/carexample/")})
	if err != nil {
		panic(err)
	}
	dagStore, err := dagstore.NewDAGStore(
		dagstore.Config{
			TransientsDir: "/tmp/transients",
			Datastore:     ds,
			MountRegistry: r,
			TopLevelIndex: index.NewInverted(namespace.Wrap(ds, invertedIndexPrefix)),
		},
	)
	if err != nil {
		panic(err)
	}
	// TODO: ctx
	err = dagStore.Start(context.Background())
	if err != nil {
		panic(err)
	}
	return &DAGBlockStore{*dagStore}
}

func (dbs *DAGBlockStore) Has(ctx context.Context, c cid.Cid) (bool, error) {
	//TODO should has return true only if the shard is available locally?
	//fmt.Println("calling has")
	keys, err := dbs.ShardsContainingMultihash(ctx, c.Hash())
	if err != nil {
		return false, err
	}
	return len(keys) > 0, nil
}

func (dbs *DAGBlockStore) Get(ctx context.Context, c cid.Cid) (blocks.Block, error) {
	//fmt.Println("calling get")
	// We might need a top level index here that points from cid to the shard that contains it
	bs, err := dbs.getReadOnlyBlockstore(ctx, c)
	if err != nil {
		return nil, ipld.ErrNotFound{Cid: c}
	}
	return bs.Get(ctx, c)
}

func (dbs *DAGBlockStore) GetSize(ctx context.Context, c cid.Cid) (int, error) {
	bs, err := dbs.getReadOnlyBlockstore(ctx, c)
	if err != nil {
		// TODO: RN this is always returning an error
		return 0, err
	}
	return bs.GetSize(ctx, c)
}

func (dbs *DAGBlockStore) Put(ctx context.Context, block blocks.Block) error {
	//  TODO create CAR
	return nil
}

func (dbs *DAGBlockStore) PutMany(ctx context.Context, blocks []blocks.Block) error {
	//TODO implement me
	return nil
}

func (dbs *DAGBlockStore) AllKeysChan(ctx context.Context) (<-chan cid.Cid, error) {
	fmt.Println("calling allkeyschan")
	//TODO implement me
	panic("implement me")
}

func (dbs *DAGBlockStore) HashOnRead(enabled bool) {
	//TODO implement me
	panic("implement me")
}

func (dbs *DAGBlockStore) DeleteBlock(context.Context, cid.Cid) error {
	//TODO implement me
	panic("implement me")
}

func (dbs *DAGBlockStore) getReadOnlyBlockstore(ctx context.Context, c cid.Cid) (dagstore.ReadBlockstore, error) {
	keys, err := dbs.ShardsContainingMultihash(ctx, c.Hash())
	if err != nil {
		return nil, err
	}
	if len(keys) > 1 {
		return nil, errors.New("found more than one shard with the provided cid")
	}
	ch := make(chan dagstore.ShardResult, 1)
	err = dbs.AcquireShard(ctx, keys[0], ch, dagstore.AcquireOpts{})
	if err != nil {
		return nil, err
	}
	result := <-ch
	return result.Accessor.Blockstore()
}
