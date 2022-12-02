package store

import (
	"context"
	headerpkg "github.com/celestiaorg/celestia-node/pkg/header"

	"github.com/celestiaorg/celestia-node/header"
	lru "github.com/hashicorp/golang-lru"
	"github.com/ipfs/go-datastore"
)

// TODO(@Wondertan): There should be a more clever way to index heights, than just storing
// HeightToHash pair... heightIndexer simply stores and cashes mappings between header Height and
// Hash.
type heightIndexer struct {
	ds    datastore.Batching
	cache *lru.ARCCache
}

// newHeightIndexer creates new heightIndexer.
func newHeightIndexer(ds datastore.Batching, indexCacheSize int) (*heightIndexer, error) {
	cache, err := lru.NewARC(indexCacheSize)
	if err != nil {
		return nil, err
	}

	return &heightIndexer{
		ds:    ds,
		cache: cache,
	}, nil
}

// HashByHeight loads a header hash corresponding to the given height.
func (hi *heightIndexer) HashByHeight(ctx context.Context, h uint64) (headerpkg.Hash, error) {
	if v, ok := hi.cache.Get(h); ok {
		return v.(headerpkg.Hash), nil
	}

	val, err := hi.ds.Get(ctx, heightKey(h))
	if err != nil {
		return nil, err
	}

	hi.cache.Add(h, headerpkg.Hash(val))
	return val, nil
}

// IndexTo saves mapping between header Height and Hash to the given batch.
func (hi *heightIndexer) IndexTo(ctx context.Context, batch datastore.Batch, headers ...*header.ExtendedHeader) error {
	for _, h := range headers {
		err := batch.Put(ctx, heightKey(uint64(h.Height())), h.Hash())
		if err != nil {
			return err
		}
	}

	return nil
}
