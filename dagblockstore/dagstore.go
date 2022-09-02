package dagblockstore

import (
	"context"
	"errors"
	"github.com/filecoin-project/dagstore"
	"github.com/filecoin-project/dagstore/index"
	"github.com/filecoin-project/dagstore/mount"
	"github.com/filecoin-project/dagstore/shard"
	lru "github.com/hashicorp/golang-lru"
	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	ipld "github.com/ipfs/go-ipld-format"
	logging "github.com/ipfs/go-log/v2"
	"os"
	"sync"
)

var _ blockstore.Blockstore = (*DAGBlockStore)(nil)

var (
	log                     = logging.Logger("dagblockstore")
	maxCacheSize            = 100
	ErrUnsupportedOperation = errors.New("unsupported operation")
	ErrMultipleShardsFound  = errors.New("found more than one shard with the provided cid")
)

type DAGStore = dagstore.DAGStore

type accessorWithBlockstore struct {
	sa *dagstore.ShardAccessor
	bs dagstore.ReadBlockstore
}

// DAGBlockStore implements the blockstore interface on a DAGStore.
// The lru cache approach is heavily inspired by the open PR filecoin-project/dagstore/116.
// The main differences to the implementation here are that we do not support multiple shards per key,
// call GetSize directly on the underlying RO blockstore, and do not throw errors on Put/PutMany.
type DAGBlockStore struct {
	DAGStore

	bsStripedLocks [256]sync.Mutex
	// caches the blockstore for a given shard for shard read affinity i.e.
	// further reads will likely be from the same shard. Maps (shard key -> blockstore).
	blockstoreCache *lru.Cache
}

// TODO: Return error. Take in cache size as parameter, added to a config.
func NewDAGBlockStore(ds datastore.Batching) *DAGBlockStore {
	// instantiate the blockstore cache
	bslru, err := lru.NewWithEvict(maxCacheSize, func(_ interface{}, val interface{}) {
		// ensure we close the blockstore for a shard when it's evicted from the cache so dagstore can gc it.
		abs := val.(*accessorWithBlockstore)
		abs.sa.Close()
	})
	if err != nil {
		panic("could not create lru cache for read only blockstores")
	}

	// create mount registry (what types of mounts are supported)
	r := mount.NewRegistry()
	err = r.Register("fs", &mount.FSMount{FS: os.DirFS("/tmp/carexample/")})

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
	return &DAGBlockStore{
		DAGStore:        *dagStore,
		blockstoreCache: bslru,
	}
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
	// TODO: if bs.Get returns an error and it is from the cache, we should remove it from the cache
	return bs.Get(ctx, c)
}

func (dbs *DAGBlockStore) GetSize(ctx context.Context, c cid.Cid) (int, error) {
	bs, err := dbs.getReadOnlyBlockstore(ctx, c)
	if err != nil {
		return 0, err
	}
	return bs.GetSize(ctx, c)
}

// Put needs to not return an error because it is called by the exchange
func (dbs *DAGBlockStore) Put(ctx context.Context, block blocks.Block) error {
	return nil
}

// PutMany needs to not return an error because it is called by the exchange
func (dbs *DAGBlockStore) PutMany(ctx context.Context, blocks []blocks.Block) error {
	return nil
}

func (dbs *DAGBlockStore) AllKeysChan(ctx context.Context) (<-chan cid.Cid, error) {
	return nil, ErrUnsupportedOperation
}

func (dbs *DAGBlockStore) HashOnRead(enabled bool) {
	panic(ErrUnsupportedOperation)
}

func (dbs *DAGBlockStore) DeleteBlock(context.Context, cid.Cid) error {
	return ErrUnsupportedOperation
}

func (dbs *DAGBlockStore) getReadOnlyBlockstore(ctx context.Context, c cid.Cid) (dagstore.ReadBlockstore, error) {
	keys, err := dbs.ShardsContainingMultihash(ctx, c.Hash())
	if err != nil {
		return nil, err
	}
	if len(keys) > 1 {
		return nil, ErrMultipleShardsFound
	}

	// try to fetch from cache
	shardKey := keys[0]
	bs, err := dbs.readFromBSCache(shardKey)
	if err == nil && bs != nil {
		return bs, nil
	}

	// wasn't found in cache, so acquire it and add to cache
	ch := make(chan dagstore.ShardResult, 1)
	err = dbs.AcquireShard(ctx, shardKey, ch, dagstore.AcquireOpts{})
	if err != nil {
		return nil, err
	}
	result := <-ch

	blockStore, err := dbs.addToBSCache(shardKey, result.Accessor)
	if err != nil {
		return nil, err
	}

	return blockStore, err
}

func (dbs *DAGBlockStore) readFromBSCache(shardContainingCid shard.Key) (dagstore.ReadBlockstore, error) {
	lk := &dbs.bsStripedLocks[shardKeyToStriped(shardContainingCid)]
	lk.Lock()
	defer lk.Unlock()

	// We've already ensured that the given shard has the cid/multihash we are looking for.
	val, ok := dbs.blockstoreCache.Get(shardContainingCid)
	if !ok {
		return nil, errors.New("not found in cache")
	}

	rbs := val.(*accessorWithBlockstore).bs
	log.Debugw("read blockstore from cache", "key", shardContainingCid)
	return rbs, nil
}

func (dbs *DAGBlockStore) addToBSCache(
	shardContainingCid shard.Key,
	accessor *dagstore.ShardAccessor,
) (dagstore.ReadBlockstore, error) {
	lk := &dbs.bsStripedLocks[shardKeyToStriped(shardContainingCid)]
	lk.Lock()
	defer lk.Unlock()

	blockStore, err := accessor.Blockstore()
	if err != nil {
		return nil, err
	}

	dbs.blockstoreCache.Add(shardContainingCid, &accessorWithBlockstore{
		bs: blockStore,
		sa: accessor,
	})
	log.Debugw("added blockstore to cache", "key", shardContainingCid)
	return blockStore, nil
}

func shardKeyToStriped(sk shard.Key) byte {
	return sk.String()[len(sk.String())-1]
}
