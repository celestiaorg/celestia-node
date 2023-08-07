package eds

import (
	"context"
	"fmt"

	"github.com/filecoin-project/dagstore/index"
	"github.com/filecoin-project/dagstore/shard"
	ds "github.com/ipfs/go-datastore"
	"github.com/multiformats/go-multihash"

	dsbadger "github.com/celestiaorg/go-ds-badger4"
)

const invertedIndexPath = "/inverted_index/"

// simpleInvertedIndex is an inverted index that only stores a single shard key per multihash. Its
// implementation is modified from the default upstream implementation in dagstore/index.
type simpleInvertedIndex struct {
	ds ds.Batching
}

// newSimpleInvertedIndex returns a new inverted index that only stores a single shard key per
// multihash. This is because we use badger as a storage backend, so updates are expensive, and we
// don't care which shard is used to serve a cid.
func newSimpleInvertedIndex(storePath string) (*simpleInvertedIndex, error) {
	opts := dsbadger.DefaultOptions // this should be copied
	// turn off value log GC
	opts.GcInterval = 0
	// 20 compactors show to have no hangups on put operation up to 40k blocks with eds size 128.
	opts.NumCompactors = 20
	// use minimum amount of NumLevelZeroTables to trigger L0 compaction faster
	opts.NumLevelZeroTables = 1

	ds, err := dsbadger.NewDatastore(storePath+invertedIndexPath, &opts)
	if err != nil {
		return nil, fmt.Errorf("can't open Badger Datastore: %w", err)
	}

	return &simpleInvertedIndex{ds: ds}, nil
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

	err = mhIter.ForEach(func(mh multihash.Multihash) error {
		key := ds.NewKey(string(mh))
		if err := batch.Put(ctx, key, []byte(sk.String())); err != nil {
			return fmt.Errorf("failed to put mh=%s, err=%w", mh, err)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to add index entry: %w", err)
	}

	if err := batch.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit batch: %w", err)
	}
	return nil
}

func (s *simpleInvertedIndex) GetShardsForMultihash(ctx context.Context, mh multihash.Multihash) ([]shard.Key, error) {
	key := ds.NewKey(string(mh))
	sbz, err := s.ds.Get(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("failed to lookup index for mh %s, err: %w", mh, err)
	}

	return []shard.Key{shard.KeyFromString(string(sbz))}, nil
}

func (s *simpleInvertedIndex) close() error {
	return s.ds.Close()
}
