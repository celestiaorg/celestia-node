package store

import (
	"context"

	lru "github.com/hashicorp/golang-lru"
	"github.com/ipfs/go-datastore"

	"github.com/celestiaorg/celestia-node/libs/header"
)

// TODO(@Wondertan): There should be a more clever way to index heights, than just storing
// HeightToHash pair... heightIndexer simply stores and cashes mappings between header Height and
// Hash.
type heightIndexer[H header.Header] struct {
	ds    datastore.Batching
	cache *lru.ARCCache
}

// newHeightIndexer creates new heightIndexer.
func newHeightIndexer[H header.Header](ds datastore.Batching, indexCacheSize int) (*heightIndexer[H], error) {
	cache, err := lru.NewARC(indexCacheSize)
	if err != nil {
		return nil, err
	}

	return &heightIndexer[H]{
		ds:    ds,
		cache: cache,
	}, nil
}

// HashByHeight loads a header hash corresponding to the given height.
func (hi *heightIndexer[H]) HashByHeight(ctx context.Context, h uint64) (header.Hash, error) {
	if v, ok := hi.cache.Get(h); ok {
		return v.(header.Hash), nil
	}

	val, err := hi.ds.Get(ctx, heightKey(h))
	if err != nil {
		return nil, err
	}

	hi.cache.Add(h, header.Hash(val))
	return val, nil
}

// IndexTo saves mapping between header Height and Hash to the given batch.
func (hi *heightIndexer[H]) IndexTo(ctx context.Context, batch datastore.Batch, headers ...H) error {
	for _, h := range headers {
		err := batch.Put(ctx, heightKey(uint64(h.Height())), h.Hash())
		if err != nil {
			return err
		}
	}

	return nil
}
