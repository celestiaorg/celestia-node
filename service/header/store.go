package header

import (
	"context"
	"fmt"
	"strconv"
	"sync"

	lru "github.com/hashicorp/golang-lru"
	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/namespace"

	"github.com/celestiaorg/celestia-core/libs/bytes"
)

// TODO(@Wondertan): Those values must be configurable and proper defaults should be set for specific node type.
var (
	// DefaultStoreCache defines the amount of max entries allowed in the Header Store cache.
	DefaultStoreCache = 1024
	// DefaultIndexCache defines the amount of max entries allowed in the Height to Hash index cache.
	DefaultIndexCache = 256
)

var (
	storePrefix = datastore.NewKey("headers")
	headKey     = datastore.NewKey("head")
)

type store struct {
	ds    datastore.Batching
	cache *lru.ARCCache
	index *heightIndexer

	headLk sync.RWMutex
	head   bytes.HexBytes
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

	return store, store.newHead(head.Hash())
}

func newStore(ds datastore.Batching) (*store, error) {
	ds = namespace.Wrap(ds, storePrefix)
	cache, err := lru.NewARC(DefaultStoreCache)
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

func (s *store) Open(context.Context) error {
	err := s.loadHead()
	if err == datastore.ErrNotFound {
		return ErrNoHead
	}

	return err
}

func (s *store) Head(ctx context.Context) (*ExtendedHeader, error) {
	s.headLk.RLock()
	head := s.head
	s.headLk.RUnlock()
	return s.Get(ctx, head)
}

func (s *store) Get(_ context.Context, hash bytes.HexBytes) (*ExtendedHeader, error) {
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

func (s *store) GetByHeight(ctx context.Context, height int64) (*ExtendedHeader, error) {
	hash, err := s.index.HashByHeight(height)
	if err != nil {
		if err == datastore.ErrNotFound {
			return nil, ErrNotFound
		}

		return nil, err
	}

	return s.Get(ctx, hash)
}

func (s *store) GetRangeByHeight(ctx context.Context, from, to int64) ([]*ExtendedHeader, error) {
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

func (s *store) Has(_ context.Context, hash bytes.HexBytes) (bool, error) {
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
	if err != nil {
		return err
	}

	verified := make([]*ExtendedHeader, 0, lh)
	for _, h := range headers {
		err = VerifyAdjacent(head, h)
		if err != nil {
			log.Errorw("invalid header", "head", head.Hash(), "header", h.Hash(), "err", err)
			break // if some headers are cryptographically valid, why not include them? Exactly, so let's include
		}
		verified, head = append(verified, h), h
	}
	if len(verified) == 0 {
		return fmt.Errorf("header/store: no valid headers were given")
	}

	err = s.put(verified...)
	if err != nil {
		return err
	}

	return s.newHead(head.Hash())
}

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

func (s *store) loadHead() error {
	s.headLk.Lock()
	defer s.headLk.Unlock()

	if s.head != nil {
		return nil
	}

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

func (s *store) newHead(head bytes.HexBytes) error {
	log.Infow("new head", "hash", head)

	// at this point Header body of the given 'head' must be already written with put
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
type heightIndexer struct {
	ds    datastore.Batching
	cache *lru.ARCCache
}

func newHeightIndexer(ds datastore.Batching) (*heightIndexer, error) {
	cache, err := lru.NewARC(DefaultIndexCache)
	if err != nil {
		return nil, err
	}

	return &heightIndexer{
		ds:    ds,
		cache: cache,
	}, nil
}

func (hi *heightIndexer) HashByHeight(h int64) (bytes.HexBytes, error) {
	if v, ok := hi.cache.Get(h); ok {
		return v.(bytes.HexBytes), nil
	}

	return hi.ds.Get(heightKey(h))
}

func (hi *heightIndexer) Index(headers ...*ExtendedHeader) error {
	batch, err := hi.ds.Batch()
	if err != nil {
		return err
	}

	for _, h := range headers {
		err := batch.Put(heightKey(h.Height), h.Hash())
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

func heightKey(h int64) datastore.Key {
	return datastore.NewKey(strconv.Itoa(int(h)))
}

func headerKey(h *ExtendedHeader) datastore.Key {
	return datastore.NewKey(h.Hash().String())
}
