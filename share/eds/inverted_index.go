package eds

import (
	"context"
	"errors"
	"fmt"
	"runtime"

	"github.com/dgraph-io/badger/v4/options"
	"github.com/filecoin-project/dagstore/index"
	"github.com/filecoin-project/dagstore/shard"
	ds "github.com/ipfs/go-datastore"
	dsbadger "github.com/ipfs/go-ds-badger4"
	"github.com/multiformats/go-multihash"
)

const invertedIndexPath = "/inverted_index/"

// ErrNotFoundInIndex is returned instead of ErrNotFound if the multihash doesn't exist in the index
var ErrNotFoundInIndex = errors.New("does not exist in index")

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
	// turn off value log GC as we don't use value log
	opts.GcInterval = 0
	// use minimum amount of NumLevelZeroTables to trigger L0 compaction faster
	opts.NumLevelZeroTables = 1
	// MaxLevels = 8 will allow the db to grow to ~11.1 TiB
	opts.MaxLevels = 8
	// inverted index stores unique hash keys, so we don't need to detect conflicts
	opts.DetectConflicts = false
	// we don't need compression for inverted index as it just hashes
	opts.Compression = options.None
	compactors := runtime.NumCPU()
	if compactors < 2 {
		compactors = 2
	}
	if compactors > opts.MaxLevels { // ensure there is no more compactors than db table levels
		compactors = opts.MaxLevels
	}
	opts.NumCompactors = compactors

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
		return nil, errors.Join(ErrNotFoundInIndex, err)
	}

	return []shard.Key{shard.KeyFromString(string(sbz))}, nil
}

func (s *simpleInvertedIndex) close() error {
	return s.ds.Close()
}
