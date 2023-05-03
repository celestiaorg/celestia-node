package eds

import (
	"context"
	"testing"

	"github.com/filecoin-project/dagstore/shard"
	"github.com/ipfs/go-datastore"
	ds_sync "github.com/ipfs/go-datastore/sync"
	"github.com/multiformats/go-multihash"
	"github.com/stretchr/testify/require"
)

type mockIterator struct {
	mhs []multihash.Multihash
}

func (m *mockIterator) ForEach(f func(mh multihash.Multihash) error) error {
	for _, mh := range m.mhs {
		if err := f(mh); err != nil {
			return err
		}
	}
	return nil
}

// TestMultihashesForShard ensures that the inverted index correctly stores a single shard key per
// duplicate multihash
func TestMultihashesForShard(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	mhs := []multihash.Multihash{
		multihash.Multihash("mh1"),
		multihash.Multihash("mh2"),
		multihash.Multihash("mh3"),
	}

	mi := &mockIterator{mhs: mhs}
	ds := ds_sync.MutexWrap(datastore.NewMapDatastore())
	invertedIndex := newSimpleInvertedIndex(ds)

	// 1. Add all 3 multihashes to shard1
	err := invertedIndex.AddMultihashesForShard(ctx, mi, shard.KeyFromString("shard1"))
	require.NoError(t, err)
	shardKeys, err := invertedIndex.GetShardsForMultihash(ctx, mhs[0])
	require.NoError(t, err)
	require.Equal(t, []shard.Key{shard.KeyFromString("shard1")}, shardKeys)

	// 2. Add mh1 to shard2, and ensure that mh1 still points to shard1
	err = invertedIndex.AddMultihashesForShard(ctx, &mockIterator{mhs: mhs[:1]}, shard.KeyFromString("shard2"))
	require.NoError(t, err)
	shardKeys, err = invertedIndex.GetShardsForMultihash(ctx, mhs[0])
	require.NoError(t, err)
	require.Equal(t, []shard.Key{shard.KeyFromString("shard1")}, shardKeys)
}
