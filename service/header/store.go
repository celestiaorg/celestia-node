package header

import (
	"context"
	"errors"

	lru "github.com/hashicorp/golang-lru"
	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/namespace"

	tmbytes "github.com/tendermint/tendermint/libs/bytes"
)

// TODO(@Wondertan): Those values must be configurable and proper defaults should be set for specific node type.
var (
	// DefaultStoreCacheSize defines the amount of max entries allowed in the Header Store cache.
	DefaultStoreCacheSize = 10240
	// DefaultIndexCacheSize defines the amount of max entries allowed in the Height to Hash index cache.
	DefaultIndexCacheSize = 2048
)

// errStoppedStore is returned on operations over stopped store
var errStoppedStore = errors.New("header: stopped store")

// store implement Store interface for ExtendedHeader over Datastore.
type store struct {
	// header storing
	//
	// underlying KV store
	ds datastore.Batching
	// adaptive replacement cache of headers
	cache *lru.ARCCache

	// header heights management
	//
	// maps heights to hashes
	heightIndex *heightIndexer
	// manages current store head height (1) and
	// allows callers to wait till header for a height is stored (2)
	heightSub *heightSub

	// writing to datastore
	//
	// queued of header ranges to be written
	writes chan []*ExtendedHeader
	// signals writes being done
	writesDn chan struct{}
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

	return store, store.Init(context.TODO(), head)
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
		ds:          ds,
		cache:       cache,
		heightIndex: index,
		heightSub:   newHeightSub(),
		writes:      make(chan []*ExtendedHeader, 8),
		writesDn:    make(chan struct{}),
	}, nil
}

func (s *store) Init(_ context.Context, initial *ExtendedHeader) error {
	// trust the given header as the initial head
	err := s.write(initial)
	if err != nil {
		return err
	}

	log.Infow("initial head", "height", initial.Height, "hash", initial.Hash())
	return nil
}

func (s *store) Start(ctx context.Context) error {
	head, err := s.readHead(ctx)
	switch err {
	default:
		return err
	case datastore.ErrNotFound:
		log.Warnf("starting uninitialized store")
	case nil:
		s.heightSub.SetHeight(uint64(head.Height))
	}

	go s.writeLoop()
	return nil
}

func (s *store) Stop(ctx context.Context) error {
	select {
	case <-s.writesDn:
		return errStoppedStore
	default:
	}
	// signal to prevent further writes to Store
	s.writes <- nil
	select {
	case <-s.writesDn: // wait till it is done writing
	case <-ctx.Done():
		return ctx.Err()
	}

	// cleanup caches
	s.cache.Purge()
	s.heightIndex.cache.Purge()
	return nil
}

func (s *store) Head(ctx context.Context) (*ExtendedHeader, error) {
	head, err := s.GetByHeight(ctx, s.heightSub.Height())
	if err == ErrNotFound {
		return nil, ErrNoHead
	}
	return head, err
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
	// if the requested 'height' was not yet published
	// we subscribe and for it
	h, err := s.heightSub.Sub(ctx, height)
	if err != errElapsedHeight {
		return h, err
	}
	// otherwise, the errElapsedHeight is thrown,
	// which means the requested 'height' should be already stored

	hash, err := s.heightIndex.HashByHeight(height)
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
	if err != nil {
		return err
	}

	verified := make([]*ExtendedHeader, 0, lh)
	for i, h := range headers {
		err = head.VerifyAdjacent(h)
		if err != nil {
			if i == 0 {
				return err
			}

			var verErr *VerifyError
			if errors.As(err, &verErr) {
				log.Errorw("invalid header",
					"height", head.Height,
					"hash", h.Hash(),
					"current height", head.Height,
					"reason", verErr.Reason)
				break
			}
		}
		verified, head = append(verified, h), h
	}

	// queue headers to be written on disk
	select {
	case s.writes <- verified:
		// once we sure the 'write' is enqueued - update caches with verified headers
		for _, h := range verified {
			s.cache.Add(h.Hash().String(), h)
			s.heightIndex.cache.Add(uint64(h.Height), h.Hash())
		}
		// and notify waiters if any + increase current height
		// it is important to do Pub after updating caches
		// so cache is consistent with atomic Height counter on the heightSub
		s.heightSub.Pub(verified...)
		// we return an error here after writing,
		// as there might be an invalid header in between of a given range
		return err
	case <-s.writesDn:
		return errStoppedStore
	case <-ctx.Done():
		return ctx.Err()
	}
}

// writeLoop performs writing task to the underlying datastore in a separate routine
// This way writes are controlled and manageable from one place allowing
// (1) Appends not to be blocked on long disk IO writes and underlying DB compactions
// (2) Batching header writes
// TODO(@Wondertan): Batch multiple writes together, but first the proper batch size should be investigated
func (s *store) writeLoop() {
	defer close(s.writesDn)
	for headers := range s.writes {
		if headers == nil {
			return
		}

		err := s.write(headers...)
		if err != nil {
			// TODO(@Wondertan): Should this be a fatal error case with os.Exit?
			from, to := uint64(headers[0].Height), uint64(headers[len(headers)-1].Height)
			log.Errorw("writing header batch", "from", from, "to", to)
		}
	}
}

// write stores the given headers on disk
func (s *store) write(headers ...*ExtendedHeader) error {
	// collect all the headers in a batch to be written
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

	// write the batch on disk
	err = batch.Commit()
	if err != nil {
		return err
	}

	// write height indexes for headers as well
	err = s.heightIndex.Index(headers...)
	if err != nil {
		return err
	}

	// marshal and store reference to the new head
	b, err := headers[len(headers)-1].Hash().MarshalJSON()
	if err != nil {
		return err
	}

	return s.ds.Put(headKey, b)
}

// readHead loads the head from the disk.
func (s *store) readHead(ctx context.Context) (*ExtendedHeader, error) {
	b, err := s.ds.Get(headKey)
	if err != nil {
		return nil, err
	}

	var head tmbytes.HexBytes
	err = head.UnmarshalJSON(b)
	if err != nil {
		return nil, err
	}

	return s.Get(ctx, head)
}
