package eds

import (
	"context"
	"errors"
	"fmt"

	"github.com/filecoin-project/dagstore"
	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	ipld "github.com/ipfs/go-ipld-format"
)

var _ blockstore.Blockstore = (*Blockstore)(nil)

var (
	errUnsupportedOperation = errors.New("unsupported operation")
	errShardNotFound        = errors.New("the provided cid does not map to any shard")
)

// Blockstore implements the blockstore.Blockstore interface on an EDSStore.
// The lru cache approach is heavily inspired by the existing implementation upstream.
// We simplified the design to not support multiple shards per key, call GetSize directly on the
// underlying RO blockstore, and do not throw errors on Put/PutMany. Also, we do not abstract away
// the blockstore operations.
//
// The intuition here is that each CAR file is its own blockstore, so we need this top level
// implementation to allow for the blockstore operations to be routed to the underlying stores.
type Blockstore struct {
	store *Store
	cache *blockstoreCache
}

func NewEDSBlockstore(store *Store, cache *blockstoreCache) *Blockstore {
	return &Blockstore{
		store: store,
		cache: cache,
	}
}

func (bs *Blockstore) Has(ctx context.Context, cid cid.Cid) (bool, error) {
	keys, err := bs.store.dgstr.ShardsContainingMultihash(ctx, cid.Hash())
	if err != nil {
		return false, fmt.Errorf("failed to find shards containing multihash: %w", err)
	}
	return len(keys) > 0, nil
}

func (bs *Blockstore) Get(ctx context.Context, cid cid.Cid) (blocks.Block, error) {
	blockstr, err := bs.getReadOnlyBlockstore(ctx, cid)
	if err != nil {
		log.Debugf("failed to get blockstore for cid %s: %s", cid, err)
		// nmt's GetNode expects an ipld.ErrNotFound when a cid is not found.
		return nil, ipld.ErrNotFound{Cid: cid}
	}
	return blockstr.Get(ctx, cid)
}

func (bs *Blockstore) GetSize(ctx context.Context, cid cid.Cid) (int, error) {
	blockstr, err := bs.getReadOnlyBlockstore(ctx, cid)
	if err != nil {
		return 0, err
	}
	return blockstr.GetSize(ctx, cid)
}

// DeleteBlock is a noop on the EDS Blockstore that returns an errUnsupportedOperation when called.
func (bs *Blockstore) DeleteBlock(context.Context, cid.Cid) error {
	return errUnsupportedOperation
}

// Put is a noop on the EDS blockstore, but it does not return an error because it is called by
// bitswap. For clarification, an implementation of Put does not make sense in this context because
// it is unclear which CAR file the block should be written to.
func (bs *Blockstore) Put(context.Context, blocks.Block) error {
	return nil
}

// PutMany is a noop on the EDS blockstore, but it does not return an error because it is called by
// bitswap. For clarification, an implementation of PutMany does not make sense in this context
// because it is unclear which CAR file the blocks should be written to.
func (bs *Blockstore) PutMany(context.Context, []blocks.Block) error {
	return nil
}

// AllKeysChan is a noop on the EDS blockstore because the keys are not stored in a single CAR file.
func (bs *Blockstore) AllKeysChan(context.Context) (<-chan cid.Cid, error) {
	return nil, errUnsupportedOperation
}

// HashOnRead is a noop on the EDS blockstore but an error cannot be returned due to the method
// signature from the blockstore interface.
func (bs *Blockstore) HashOnRead(bool) {
	log.Warnf("HashOnRead is a noop on the EDS blockstore")
}

// getReadOnlyBlockstore finds the underlying blockstore of the shard that contains the given CID.
func (bs *Blockstore) getReadOnlyBlockstore(ctx context.Context, cid cid.Cid) (dagstore.ReadBlockstore, error) {
	keys, err := bs.store.dgstr.ShardsContainingMultihash(ctx, cid.Hash())
	if err != nil {
		return nil, fmt.Errorf("failed to find shards containing multihash: %w", err)
	}
	if len(keys) == 0 {
		return nil, errShardNotFound
	}

	// a share can exist in multiple EDSes, so just take the first one.
	shardKey := keys[0]
	accessor, err := bs.store.getAccessor(ctx, shardKey)
	if err != nil {
		return nil, fmt.Errorf("failed to get accessor for shard %s: %w", shardKey, err)
	}
	return accessor.bs, nil
}
