package header

import (
	lru "github.com/hashicorp/golang-lru"
	"github.com/ipfs/go-datastore"
	tmbytes "github.com/tendermint/tendermint/libs/bytes"
)

// TODO(@Wondertan): There should be a more clever way to index heights, than just storing HeightToHash pair...
// heightIndexer simply stores and cashes mappings between header Height and Hash.
type heightIndexer struct {
	ds    datastore.Batching
	cache *lru.ARCCache
}

// newHeightIndexer creates new heightIndexer.
func newHeightIndexer(ds datastore.Batching) (*heightIndexer, error) {
	cache, err := lru.NewARC(DefaultIndexCacheSize)
	if err != nil {
		return nil, err
	}

	return &heightIndexer{
		ds:    ds,
		cache: cache,
	}, nil
}

// HashByHeight loads a header hash corresponding to the given height.
func (hi *heightIndexer) HashByHeight(h uint64) (tmbytes.HexBytes, error) {
	if v, ok := hi.cache.Get(h); ok {
		return v.(tmbytes.HexBytes), nil
	}

	return hi.ds.Get(heightKey(h))
}

// IndexTo saves mapping between header Height and Hash to the given batch.
func (hi *heightIndexer) IndexTo(batch datastore.Batch, headers ...*ExtendedHeader) error {
	for _, h := range headers {
		err := batch.Put(heightKey(uint64(h.Height)), h.Hash())
		if err != nil {
			return err
		}
	}

	return nil
}
