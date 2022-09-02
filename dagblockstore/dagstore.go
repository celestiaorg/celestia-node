package dagblockstore

import (
	"context"
	"errors"
	"github.com/filecoin-project/dagstore"
	"github.com/filecoin-project/dagstore/index"
	"github.com/filecoin-project/dagstore/mount"
	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	ipld "github.com/ipfs/go-ipld-format"
	logging "github.com/ipfs/go-log/v2"
	"os"
)

var (
	_   blockstore.Blockstore = (*DAGBlockStore)(nil)
	log                       = logging.Logger("dagblockstore")
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
			TopLevelIndex: index.NewInverted(ds),
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
	//err = logging.SetLogLevel("dagblockstore", "debug")
	//if err != nil {
	//	panic(err)
	//}
	return &DAGBlockStore{*dagStore}
}

func (dbs *DAGBlockStore) Has(ctx context.Context, c cid.Cid) (bool, error) {
	keys, err := dbs.ShardsContainingMultihash(ctx, c.Hash())
	if err != nil {
		return false, err
	}
	return len(keys) > 0, nil
}

func (dbs *DAGBlockStore) Get(ctx context.Context, c cid.Cid) (blocks.Block, error) {
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
	return nil
}

func (dbs *DAGBlockStore) PutMany(ctx context.Context, blocks []blocks.Block) error {
	//TODO implement me
	return nil
}

func (dbs *DAGBlockStore) AllKeysChan(ctx context.Context) (<-chan cid.Cid, error) {
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
