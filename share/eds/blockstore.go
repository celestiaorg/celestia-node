package eds

import (
	"context"
	"errors"
	"fmt"

	bstore "github.com/ipfs/boxo/blockstore"
	"github.com/ipfs/boxo/datastore/dshelp"
	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/namespace"
	ipld "github.com/ipfs/go-ipld-format"
)

var _ bstore.Blockstore = (*blockstore)(nil)

var (
	blockstoreCacheKey      = datastore.NewKey("bs-cache")
	errUnsupportedOperation = errors.New("unsupported operation")
)

// blockstore implements the store.Blockstore interface on an EDSStore.
// The lru cache approach is heavily inspired by the existing implementation upstream.
// We simplified the design to not support multiple shards per key, call GetSize directly on the
// underlying RO blockstore, and do not throw errors on Put/PutMany. Also, we do not abstract away
// the blockstore operations.
//
// The intuition here is that each CAR file is its own blockstore, so we need this top level
// implementation to allow for the blockstore operations to be routed to the underlying stores.
type blockstore struct {
	store *Store
	ds    datastore.Batching
}

func newBlockstore(store *Store, ds datastore.Batching) *blockstore {
	return &blockstore{
		store: store,
		ds:    namespace.Wrap(ds, blockstoreCacheKey),
	}
}

func (bs *blockstore) Has(ctx context.Context, cid cid.Cid) (bool, error) {
	keys, err := bs.store.dgstr.ShardsContainingMultihash(ctx, cid.Hash())
	if errors.Is(err, ErrNotFound) || errors.Is(err, ErrNotFoundInIndex) {
		// key wasn't found in top level blockstore, but could be in datastore while being reconstructed
		dsHas, dsErr := bs.ds.Has(ctx, dshelp.MultihashToDsKey(cid.Hash()))
		if dsErr != nil {
			return false, nil
		}
		return dsHas, nil
	}
	if err != nil {
		return false, err
	}

	return len(keys) > 0, nil
}

func (bs *blockstore) Get(ctx context.Context, cid cid.Cid) (blocks.Block, error) {
	blockstr, err := bs.getReadOnlyBlockstore(ctx, cid)
	if err == nil {
		defer closeAndLog("blockstore", blockstr)
		return blockstr.Get(ctx, cid)
	}

	if errors.Is(err, ErrNotFound) || errors.Is(err, ErrNotFoundInIndex) {
		k := dshelp.MultihashToDsKey(cid.Hash())
		blockData, err := bs.ds.Get(ctx, k)
		if err == nil {
			return blocks.NewBlockWithCid(blockData, cid)
		}
		// nmt's GetNode expects an ipld.ErrNotFound when a cid is not found.
		return nil, ipld.ErrNotFound{Cid: cid}
	}

	log.Debugf("failed to get blockstore for cid %s: %s", cid, err)
	return nil, err
}

func (bs *blockstore) GetSize(ctx context.Context, cid cid.Cid) (int, error) {
	blockstr, err := bs.getReadOnlyBlockstore(ctx, cid)
	if err == nil {
		defer closeAndLog("blockstore", blockstr)
		return blockstr.GetSize(ctx, cid)
	}

	if errors.Is(err, ErrNotFound) || errors.Is(err, ErrNotFoundInIndex) {
		k := dshelp.MultihashToDsKey(cid.Hash())
		size, err := bs.ds.GetSize(ctx, k)
		if err == nil {
			return size, nil
		}
		// nmt's GetSize expects an ipld.ErrNotFound when a cid is not found.
		return 0, ipld.ErrNotFound{Cid: cid}
	}

	log.Debugf("failed to get size for cid %s: %s", cid, err)
	return 0, err
}

func (bs *blockstore) DeleteBlock(ctx context.Context, cid cid.Cid) error {
	k := dshelp.MultihashToDsKey(cid.Hash())
	return bs.ds.Delete(ctx, k)
}

func (bs *blockstore) Put(ctx context.Context, blk blocks.Block) error {
	k := dshelp.MultihashToDsKey(blk.Cid().Hash())
	// note: we leave duplicate resolution to the underlying datastore
	return bs.ds.Put(ctx, k, blk.RawData())
}

func (bs *blockstore) PutMany(ctx context.Context, blocks []blocks.Block) error {
	if len(blocks) == 1 {
		// performance fast-path
		return bs.Put(ctx, blocks[0])
	}

	t, err := bs.ds.Batch(ctx)
	if err != nil {
		return err
	}
	for _, b := range blocks {
		k := dshelp.MultihashToDsKey(b.Cid().Hash())
		err = t.Put(ctx, k, b.RawData())
		if err != nil {
			return err
		}
	}
	return t.Commit(ctx)
}

// AllKeysChan is a noop on the EDS blockstore because the keys are not stored in a single CAR file.
func (bs *blockstore) AllKeysChan(context.Context) (<-chan cid.Cid, error) {
	return nil, errUnsupportedOperation
}

// HashOnRead is a noop on the EDS blockstore but an error cannot be returned due to the method
// signature from the blockstore interface.
func (bs *blockstore) HashOnRead(bool) {
	log.Warnf("HashOnRead is a noop on the EDS blockstore")
}

// getReadOnlyBlockstore finds the underlying blockstore of the shard that contains the given CID.
func (bs *blockstore) getReadOnlyBlockstore(ctx context.Context, cid cid.Cid) (*BlockstoreCloser, error) {
	keys, err := bs.store.dgstr.ShardsContainingMultihash(ctx, cid.Hash())
	if errors.Is(err, datastore.ErrNotFound) || errors.Is(err, ErrNotFoundInIndex) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to find shards containing multihash: %w", err)
	}

	// check if either cache contains an accessor
	shardKey := keys[0]
	accessor, err := bs.store.cache.Load().Get(shardKey)
	if err == nil {
		return blockstoreCloser(accessor)
	}

	// load accessor to the blockstore cache and use it as blockstoreCloser
	accessor, err = bs.store.cache.Load().Second().GetOrLoad(ctx, shardKey, bs.store.getAccessor)
	if err != nil {
		return nil, fmt.Errorf("failed to get accessor for shard %s: %w", shardKey, err)
	}
	return blockstoreCloser(accessor)
}
