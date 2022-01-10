package header

import (
	"bytes"
	"context"
	"strconv"
	"sync"

	lru "github.com/hashicorp/golang-lru"
	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/namespace"

	tmbytes "github.com/tendermint/tendermint/libs/bytes"
)

// TODO(@Wondertan): Those values must be configurable and proper defaults should be set for specific node type.
var (
	// DefaultStoreCache defines the amount of max entries allowed in the Header Store cache.
	DefaultStoreCacheSize = 1024
	// DefaultIndexCache defines the amount of max entries allowed in the Height to Hash index cache.
	DefaultIndexCacheSize = 256
)

type store struct {
	ds    datastore.Batching
	cache *lru.ARCCache
	index *heightIndexer

	headLk sync.RWMutex
	head   tmbytes.HexBytes
}

// NewStore constructs a Store over datastore.
// The datastore must have a head there otherwise Start will error.
// For first initialization of Store use NewStoreWithHead.
func NewStore(ds datastore.Batching) (Store, error) {
	return newStore(ds)
}

// NewStoreWithHead initiates a new Store and forcefully sets a given trusted header as head.
func NewStoreWithHead(ds datastore.Batching, head *ExtendedHeader) (Store, error) {
	store, err := newStore(ds)
	if err != nil {
		return nil, err
	}

	err = store.put(head)
	if err != nil {
		return nil, err
	}

	err = store.newHead(head.Hash())
	if err != nil {
		return nil, err
	}

	log.Infow("new head", "height", head.Height, "hash", head.Hash())
	return store, nil
}

func newStore(ds datastore.Batching) (*store, error) {
	ds = namespace.Wrap(ds, storePrefix)
	cache, err := lru.NewARC(DefaultStoreCacheSize)
	if err != nil {
		return nil, err
	}

	index, err := newHeightIndexer(ds)
	if err != nil {
		return nil, err
	}

	return &store{
		ds:    ds,
		cache: cache,
		index: index,
	}, nil
}

func (s *store) Head(ctx context.Context) (*ExtendedHeader, error) {
	s.headLk.RLock()
	head := s.head
	s.headLk.RUnlock()
	if head != nil {
		return s.Get(ctx, head)
	}

	err := s.loadHead()
	switch err {
	default:
		return nil, err
	case datastore.ErrNotFound:
		return nil, ErrNoHead
	case nil:
		return s.Get(ctx, s.head)
	}
}

func (s *store) Get(_ context.Context, hash tmbytes.HexBytes) (*ExtendedHeader, error) {
	if v, ok := s.cache.Get(hash.String()); ok {
		return v.(*ExtendedHeader), nil
	}

	b, err := s.ds.Get(datastore.NewKey(hash.String()))
	if err != nil {
		if err == datastore.ErrNotFound {
			return nil, ErrNotFound
		}

		return nil, err
	}

	return UnmarshalExtendedHeader(b)
}

func (s *store) GetByHeight(ctx context.Context, height uint64) (*ExtendedHeader, error) {
	hash, err := s.index.HashByHeight(height)
	if err != nil {
		if err == datastore.ErrNotFound {
			return nil, ErrNotFound
		}

		return nil, err
	}

	return s.Get(ctx, hash)
}

func (s *store) GetRangeByHeight(ctx context.Context, from, to uint64) ([]*ExtendedHeader, error) {
	h, err := s.GetByHeight(ctx, to-1)
	if err != nil {
		return nil, err
	}

	ln := to - from
	headers := make([]*ExtendedHeader, ln)
	for i := ln - 1; i > 0; i-- {
		headers[i] = h
		h, err = s.Get(ctx, h.LastHeader())
		if err != nil {
			return nil, err
		}
	}
	headers[0] = h

	return headers, nil
}

func (s *store) Has(_ context.Context, hash tmbytes.HexBytes) (bool, error) {
	if ok := s.cache.Contains(hash.String()); ok {
		return ok, nil
	}

	return s.ds.Has(datastore.NewKey(hash.String()))
}

func (s *store) Append(ctx context.Context, headers ...*ExtendedHeader) error {
	lh := len(headers)
	if lh == 0 {
		return nil
	}

	head, err := s.Head(ctx)
	switch err {
	default:
		return err
	case ErrNoHead:
		// trust the given header as the initial head
		err = s.put(headers...)
		if err != nil {
			return err
		}

		head = headers[len(headers)-1]
		err = s.newHead(head.Hash())
		if err != nil {
			return err
		}

		log.Infow("new head", "height", head.Height, "hash", head.Hash())
		return nil
	case nil:
	}

	verified := make([]*ExtendedHeader, 0, lh)
	for _, h := range headers {
		if head.Height == h.Height && bytes.Equal(head.Hash(), h.Hash()) {
			log.Warnw("duplicate header", "hash", head.Hash())
			continue
		}
		// TODO(@Wondertan): Fork choice rule - if two headers are on the same height, but have different hashes
		//  choose one who has more consensus(power) over it.
		//  It is not critical to implement this rn, because we can reliably apply the last header who has +2/3.

		err = VerifyAdjacent(head, h)
		if err != nil {
			log.Errorw("invalid header", "current head", head.Hash(), "height",
				head.Height, "attempted new header", h.Hash(), "height", h.Height, "err", err)
			break // if some headers are cryptographically valid, why not include them? Exactly, so let's include
		}
		verified, head = append(verified, h), h
	}
	if len(verified) == 0 {
		log.Warn("header/store: no valid headers were given")
		return nil
	}

	err = s.put(verified...)
	if err != nil {
		return err
	}

	err = s.newHead(head.Hash())
	if err != nil {
		return err
	}

	log.Infow("new head", "height", head.Height, "hash", head.Hash())
	return nil
}

// put saves the given headers on disk and into cache.
func (s *store) put(headers ...*ExtendedHeader) error {
	batch, err := s.ds.Batch()
	if err != nil {
		return err
	}

	for _, h := range headers {
		b, err := h.MarshalBinary()
		if err != nil {
			return err
		}

		err = batch.Put(headerKey(h), b)
		if err != nil {
			return err
		}
	}

	err = batch.Commit()
	if err != nil {
		return err
	}

	// consistency is important, so change the cache and the head only after the data is on disk
	for _, h := range headers {
		s.cache.Add(h.Hash().String(), h)
	}

	return s.index.Index(headers...)
}

// loadHead load the head hash from the disk.
func (s *store) loadHead() error {
	s.headLk.Lock()
	defer s.headLk.Unlock()

	b, err := s.ds.Get(headKey)
	if err != nil {
		return err
	}

	err = s.head.UnmarshalJSON(b)
	if err != nil {
		return err
	}

	log.Infow("loaded head", "hash", s.head)
	return nil
}

// newHead sets a new 'head' and saves it on disk.
// At this point Header body of the given 'head' must be already written with put.
func (s *store) newHead(head tmbytes.HexBytes) error {
	s.headLk.Lock()
	s.head = head
	s.headLk.Unlock()

	b, err := head.MarshalJSON()
	if err != nil {
		return err
	}

	return s.ds.Put(headKey, b)
}

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

// HashByHeight loads a header by the given height.
func (hi *heightIndexer) HashByHeight(h uint64) (tmbytes.HexBytes, error) {
	if v, ok := hi.cache.Get(h); ok {
		return v.(tmbytes.HexBytes), nil
	}

	return hi.ds.Get(heightKey(h))
}

// Index saves mapping between header Height and Hash.
func (hi *heightIndexer) Index(headers ...*ExtendedHeader) error {
	batch, err := hi.ds.Batch()
	if err != nil {
		return err
	}

	for _, h := range headers {
		err := batch.Put(heightKey(uint64(h.Height)), h.Hash())
		if err != nil {
			return err
		}
	}

	err = batch.Commit()
	if err != nil {
		return err
	}

	// update the cache only after indexes are written to the disk
	for _, h := range headers {
		hi.cache.Add(h.Height, h.Hash())
	}
	return nil
}

var (
	storePrefix = datastore.NewKey("headers")
	headKey     = datastore.NewKey("head")
)

func heightKey(h uint64) datastore.Key {
	return datastore.NewKey(strconv.Itoa(int(h)))
}

func headerKey(h *ExtendedHeader) datastore.Key {
	return datastore.NewKey(h.Hash().String())
}
