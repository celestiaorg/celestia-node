package eds

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/filecoin-project/dagstore/index"
	"github.com/filecoin-project/dagstore/shard"
	ds "github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/namespace"
	"github.com/multiformats/go-multihash"
)

// simpleInvertedIndex is an inverted index that only stores a single shard key per multihash. Its
// implementation is modified from the default upstream implementation in dagstore/index.
type simpleInvertedIndex struct {
	ds ds.Batching
}

// newSimpleInvertedIndex returns a new inverted index that only stores a single shard key per
// multihash. This is because we use badger as a storage backend, so updates are expensive, and we
// don't care which shard is used to serve a cid.
func newSimpleInvertedIndex(dts ds.Batching) *simpleInvertedIndex {
	return &simpleInvertedIndex{
		ds: namespace.Wrap(dts, ds.NewKey("/inverted/index")),
	}
}

func (s *simpleInvertedIndex) AddMultihashesForShard(
	ctx context.Context,
	mhIter index.MultihashIterator,
	sk shard.Key,
) error {
	// in the original implementation, a mutex is used here to prevent unnecessary updates to the
	// key. The amount of extra data produced by this is negligible, and the performance benefits
	// from removing the lock are significant (indexing is a hot path during sync).
	batch, err := s.ds.Batch(ctx)
	if err != nil {
		return fmt.Errorf("failed to create ds batch: %w", err)
	}

	if err := mhIter.ForEach(func(mh multihash.Multihash) error {
		key := ds.NewKey(string(mh))
		ok, err := s.ds.Has(ctx, key)
		if err != nil {
			return fmt.Errorf("failed to check if value for multihash exists %s, err: %w", mh, err)
		}

		if !ok {
			bz, err := json.Marshal(sk)
			if err != nil {
				return fmt.Errorf("failed to marshal shard key to bytes: %w", err)
			}
			if err := batch.Put(ctx, key, bz); err != nil {
				return fmt.Errorf("failed to put mh=%s, err=%w", mh, err)
			}
		}

		return nil
	}); err != nil {
		return fmt.Errorf("failed to add index entry: %w", err)
	}

	if err := batch.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit batch: %w", err)
	}

	if err := s.ds.Sync(ctx, ds.Key{}); err != nil {
		return fmt.Errorf("failed to sync puts: %w", err)
	}
	return nil
}

func (s *simpleInvertedIndex) GetShardsForMultihash(ctx context.Context, mh multihash.Multihash) ([]shard.Key, error) {
	key := ds.NewKey(string(mh))
	sbz, err := s.ds.Get(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("failed to lookup index for mh %s, err: %w", mh, err)
	}

	var shardKey shard.Key
	if err := json.Unmarshal(sbz, &shardKey); err != nil {
		return nil, fmt.Errorf("failed to unmarshal shard key for mh=%s, err=%w", mh, err)
	}

	return []shard.Key{shardKey}, nil
}
