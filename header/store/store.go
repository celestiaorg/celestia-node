package store

import (
	"context"
	"errors"
	"fmt"
	"sync"

	logging "github.com/ipfs/go-log/v2"

	lru "github.com/hashicorp/golang-lru"
	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/namespace"
	tmbytes "github.com/tendermint/tendermint/libs/bytes"

	"github.com/celestiaorg/celestia-node/header"
)

var log = logging.Logger("header/store")

// TODO(@Wondertan): Those values must be configurable and proper defaults should be set for specific node type. (#709)
var (
	// DefaultStoreCacheSize defines the amount of max entries allowed in the Header Store cache.
	DefaultStoreCacheSize = 4096
	// DefaultIndexCacheSize defines the amount of max entries allowed in the Height to Hash index cache.
	DefaultIndexCacheSize = 16384
	// DefaultWriteBatchSize defines the size of the batched header write.
	// Headers are written in batches not to thrash the underlying Datastore with writes.
	DefaultWriteBatchSize = 2048
)

var (
	// errStoppedStore is returned for attempted operations on a stopped store
	errStoppedStore = errors.New("stopped store")
)

// store implements the Store interface for ExtendedHeaders over Datastore.
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
	// manages current store read head height (1) and
	// allows callers to wait until header for a height is stored (2)
	heightSub *heightSub

	// writing to datastore
	//
	writeLk sync.Mutex
	// queue of headers to be written
	writes chan []*header.ExtendedHeader
	// signals when writes are finished
	writesDn chan struct{}
	// writeHead maintains the current write head
	writeHead *header.ExtendedHeader
	// pending keeps headers pending to be written in one batch
	pending *batch
}

// NewStore constructs a Store over datastore.
// The datastore must have a head there otherwise Start will error.
// For first initialization of Store use NewStoreWithHead.
func NewStore(ds datastore.Batching) (header.Store, error) {
	return newStore(ds)
}

// NewStoreWithHead initiates a new Store and forcefully sets a given trusted header as head.
func NewStoreWithHead(ctx context.Context, ds datastore.Batching, head *header.ExtendedHeader) (header.Store, error) {
	store, err := newStore(ds)
	if err != nil {
		return nil, err
	}

	return store, store.Init(ctx, head)
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
		writes:      make(chan []*header.ExtendedHeader, 16),
		writesDn:    make(chan struct{}),
		pending:     newBatch(DefaultWriteBatchSize),
	}, nil
}

func (s *store) Init(ctx context.Context, initial *header.ExtendedHeader) error {
	// trust the given header as the initial head
	err := s.flush(ctx, initial)
	if err != nil {
		return err
	}

	log.Infow("initialized head", "height", initial.Height, "hash", initial.Hash())
	return nil
}

func (s *store) Start(context.Context) error {
	go s.flushLoop()
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

func (s *store) Height() uint64 {
	return s.heightSub.Height()
}

func (s *store) Head(ctx context.Context) (*header.ExtendedHeader, error) {
	head, err := s.GetByHeight(ctx, s.heightSub.Height())
	if err == nil {
		return head, nil
	}

	head, err = s.readHead(ctx)
	switch err {
	default:
		return nil, err
	case datastore.ErrNotFound, header.ErrNotFound:
		return nil, header.ErrNoHead
	case nil:
		s.heightSub.SetHeight(uint64(head.Height))
		log.Infow("loaded head", "height", head.Height, "hash", head.Hash())
		return head, nil
	}
}

func (s *store) Get(ctx context.Context, hash tmbytes.HexBytes) (*header.ExtendedHeader, error) {
	if v, ok := s.cache.Get(hash.String()); ok {
		return v.(*header.ExtendedHeader), nil
	}
	// check if the requested header is not yet written on disk
	if h := s.pending.Get(hash); h != nil {
		return h, nil
	}

	b, err := s.ds.Get(ctx, datastore.NewKey(hash.String()))
	if err != nil {
		if err == datastore.ErrNotFound {
			return nil, header.ErrNotFound
		}

		return nil, err
	}

	h, err := header.UnmarshalExtendedHeader(b)
	if err != nil {
		return nil, err
	}

	s.cache.Add(h.Hash().String(), h)
	return h, nil
}

func (s *store) GetByHeight(ctx context.Context, height uint64) (*header.ExtendedHeader, error) {
	if height == 0 {
		return nil, fmt.Errorf("header/store: height must be bigger than zero")
	}
	// if the requested 'height' was not yet published
	// we subscribe to it
	h, err := s.heightSub.Sub(ctx, height)
	if err != errElapsedHeight {
		return h, err
	}
	// otherwise, the errElapsedHeight is thrown,
	// which means the requested 'height' should be present
	//
	// check if the requested header is not yet written on disk
	if h := s.pending.GetByHeight(height); h != nil {
		return h, nil
	}

	hash, err := s.heightIndex.HashByHeight(ctx, height)
	if err != nil {
		if err == datastore.ErrNotFound {
			return nil, header.ErrNotFound
		}

		return nil, err
	}

	return s.Get(ctx, hash)
}

func (s *store) GetRangeByHeight(ctx context.Context, from, to uint64) ([]*header.ExtendedHeader, error) {
	h, err := s.GetByHeight(ctx, to-1)
	if err != nil {
		return nil, err
	}

	ln := to - from
	headers := make([]*header.ExtendedHeader, ln)
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

func (s *store) Has(ctx context.Context, hash tmbytes.HexBytes) (bool, error) {
	if ok := s.cache.Contains(hash.String()); ok {
		return ok, nil
	}
	// check if the requested header is not yet written on disk
	if ok := s.pending.Has(hash); ok {
		return ok, nil
	}

	return s.ds.Has(ctx, datastore.NewKey(hash.String()))
}

func (s *store) Append(ctx context.Context, headers ...*header.ExtendedHeader) (int, error) {
	lh := len(headers)
	if lh == 0 {
		return 0, nil
	}
	// taking the ownership of append
	// mainly, this is need to avoid race conditions for s.writeHead
	s.writeLk.Lock()
	defer s.writeLk.Unlock()

	// take current write head to verify headers against
	var err error
	head := s.writeHead
	if head == nil {
		head, err = s.Head(ctx)
		if err != nil {
			return 0, err
		}
	}

	// collect valid headers
	verified := make([]*header.ExtendedHeader, 0, lh)
	for i, h := range headers {
		err = head.VerifyAdjacent(h)
		if err != nil {
			var verErr *header.VerifyError
			if errors.As(err, &verErr) {
				log.Errorw("invalid header",
					"height_of_head", head.Height,
					"hash_of_head", head.Hash(),
					"height_of_invalid", h.Height,
					"hash_of_invalid", h.Hash(),
					"reason", verErr.Reason)
			}
			// if the first header is invalid, no need to go further
			if i == 0 {
				// and simply return
				return 0, err
			}
			// otherwise, stop the loop and apply headers appeared to be valid
			break
		}
		verified, head = append(verified, h), h
	}

	// queue headers to be written on disk
	select {
	case s.writes <- verified:
		ln := len(verified)
		s.writeHead = verified[ln-1]
		log.Infow("new head", "height", s.writeHead.Height, "hash", s.writeHead.Hash())
		// we return an error here after writing,
		// as there might be an invalid header in between of a given range
		return ln, err
	case <-s.writesDn:
		return 0, errStoppedStore
	case <-ctx.Done():
		return 0, ctx.Err()
	}
}

// flushLoop performs writing task to the underlying datastore in a separate routine
// This way writes are controlled and manageable from one place allowing
// (1) Appends not to be blocked on long disk IO writes and underlying DB compactions
// (2) Batching header writes
func (s *store) flushLoop() {
	defer close(s.writesDn)
	ctx := context.Background()
	for headers := range s.writes {
		// add headers to the pending and ensure they are accessible
		s.pending.Append(headers...)
		// and notify waiters if any + increase current read head height
		// it is important to do Pub after updating pending
		// so pending is consistent with atomic Height counter on the heightSub
		s.heightSub.Pub(headers...)
		// don't flush and continue if pending batch is not grown enough,
		// and Store is not stopping(headers == nil)
		if s.pending.Len() < DefaultWriteBatchSize && headers != nil {
			continue
		}

		err := s.flush(ctx, s.pending.GetAll()...)
		if err != nil {
			// TODO(@Wondertan): Should this be a fatal error case with os.Exit?
			from, to := uint64(headers[0].Height), uint64(headers[len(headers)-1].Height)
			log.Errorw("writing header batch", "from", from, "to", to)
			continue
		}
		// reset pending
		s.pending.Reset()

		if headers == nil {
			// a signal to stop
			return
		}
	}
}

// flush writes the given batch to datastore.
func (s *store) flush(ctx context.Context, headers ...*header.ExtendedHeader) error {
	ln := len(headers)
	if ln == 0 {
		return nil
	}

	batch, err := s.ds.Batch(ctx)
	if err != nil {
		return err
	}

	// collect all the headers in the batch to be written
	for _, h := range headers {
		b, err := h.MarshalBinary()
		if err != nil {
			return err
		}

		err = batch.Put(ctx, headerKey(h), b)
		if err != nil {
			return err
		}
	}

	// marshal and add to batch reference to the new head
	b, err := headers[ln-1].Hash().MarshalJSON()
	if err != nil {
		return err
	}

	err = batch.Put(ctx, headKey, b)
	if err != nil {
		return err
	}

	// write height indexes for headers as well
	err = s.heightIndex.IndexTo(ctx, batch, headers...)
	if err != nil {
		return err
	}

	// finally, commit the batch on disk
	return batch.Commit(ctx)
}

// readHead loads the head from the datastore.
func (s *store) readHead(ctx context.Context) (*header.ExtendedHeader, error) {
	b, err := s.ds.Get(ctx, headKey)
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
