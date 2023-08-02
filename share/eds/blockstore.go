package eds

import (
	"context"
	"errors"
	"fmt"

	"github.com/filecoin-project/dagstore"
	bstore "github.com/ipfs/boxo/blockstore"
	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/namespace"
	bstore "github.com/ipfs/go-ipfs-blockstore"
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
	cache *blockstoreCache
	ds    datastore.Batching
}

func newBlockstore(store *Store, cache *blockstoreCache, ds datastore.Batching) *blockstore {
	return &blockstore{
		store: store,
		cache: cache,
		ds:    namespace.Wrap(ds, blockstoreCacheKey),
	}
}

func (bs *blockstore) Has(ctx context.Context, cid cid.Cid) (bool, error) {
	keys, err := bs.store.dgstr.ShardsContainingMultihash(ctx, cid.Hash())
	if errors.Is(err, ErrNotFound) {
		return bs.ds.Has(ctx, datastore.NewKey(cid.KeyString()))
	}
	if err != nil {
		return false, fmt.Errorf("failed to find shards containing multihash: %w", err)
	}
	return len(keys) > 0, nil
}

func (bs *blockstore) Get(ctx context.Context, cid cid.Cid) (blocks.Block, error) {
	blockstr, err := bs.getReadOnlyBlockstore(ctx, cid)
	if errors.Is(err, ErrNotFound) {
		// TODO(@distractedm1nd): Not sure if we should log the error or not. I don't think it needs
		// to be returned, since this ds.Get is a "last ditch effort" to find the block, and the
		// relevant error stays ipld.ErrNotFound
		blockData, err := bs.ds.Get(ctx, datastore.NewKey(cid.KeyString()))
		if err == nil {
			return blocks.NewBlockWithCid(blockData, cid)
		}
		// nmt's GetNode expects an ipld.ErrNotFound when a cid is not found.
		return nil, ipld.ErrNotFound{Cid: cid}
	}
	if err != nil {
		log.Debugf("failed to get Blockstore for cid %s: %s", cid, err)
		return nil, err
	}
	return blockstr.Get(ctx, cid)
}

// TODO(@distractedm1nd): Figure out why GetSize is used and if we should add datastore retrieval...
// wait, isn't this a constant?
func (bs *blockstore) GetSize(ctx context.Context, cid cid.Cid) (int, error) {
	blockstr, err := bs.getReadOnlyBlockstore(ctx, cid)
	if errors.Is(err, ErrNotFound) {
		// nmt's GetSize expects an ipld.ErrNotFound when a cid is not found.
		return 0, ipld.ErrNotFound{Cid: cid}
	}
	if err != nil {
		return 0, err
	}
	return blockstr.GetSize(ctx, cid)
}

func (bs *blockstore) DeleteBlock(ctx context.Context, cid cid.Cid) error {
	return bs.ds.Delete(ctx, datastore.NewKey(cid.KeyString()))
}

func (bs *blockstore) Put(ctx context.Context, blk blocks.Block) error {
	return bs.ds.Put(ctx, datastore.NewKey(blk.Cid().KeyString()), blk.RawData())
}

func (bs *blockstore) PutMany(ctx context.Context, blocks []blocks.Block) error {
	var err error
	for _, blk := range blocks {
		err = bs.Put(ctx, blk)
		if err != nil {
			return err
		}
	}
	return err
}

// AllKeysChan is a noop on the EDS blockstore because the keys are not stored in a single CAR file.
func (bs *blockstore) AllKeysChan(context.Context) (<-chan cid.Cid, error) {
	return nil, errUnsupportedOperation
}

// HashOnRead is a noop on the EDS blockstore but an error cannot be returned due to the method
// signature from the blockstore interface.
func (bs *blockstore) HashOnRead(bool) {
	log.Warnf("HashOnRead is a noop on the EDS Blockstore")
}

// getReadOnlyBlockstore finds the underlying blockstore of the shard that contains the given CID.
func (bs *blockstore) getReadOnlyBlockstore(ctx context.Context, cid cid.Cid) (dagstore.ReadBlockstore, error) {
	keys, err := bs.store.dgstr.ShardsContainingMultihash(ctx, cid.Hash())
	if errors.Is(err, datastore.ErrNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to find shards containing multihash: %w", err)
	}

	// a share can exist in multiple EDSes, so just take the first one.
	shardKey := keys[0]
	accessor, err := bs.store.getCachedAccessor(ctx, shardKey)
	if err != nil {
		return nil, fmt.Errorf("failed to get accessor for shard %s: %w", shardKey, err)
	}
	return accessor.bs, nil
}
