package store

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"

	lru "github.com/hashicorp/golang-lru"
	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/namespace"
	logging "github.com/ipfs/go-log/v2"

	"github.com/celestiaorg/celestia-node/libs/header"
)

var log = logging.Logger("header/store")

var (
	// errStoppedStore is returned for attempted operations on a stopped store
	errStoppedStore = errors.New("stopped store")
)

// Store implements the Store interface for Headers over Datastore.
type Store[H header.Header] struct {
	// header storing
	//
	// underlying KV store
	ds datastore.Batching
	// adaptive replacement cache of headers
	cache *lru.ARCCache

	// header heights management
	//
	// maps heights to hashes
	heightIndex *heightIndexer[H]
	// manages current store read head height (1) and
	// allows callers to wait until header for a height is stored (2)
	heightSub *heightSub[H]

	// writing to datastore
	//
	// queue of headers to be written
	writes chan []H
	// signals when writes are finished
	writesDn chan struct{}
	// writeHead maintains the current write head
	writeHead atomic.Pointer[H]
	// pending keeps headers pending to be written in one batch
	pending *batch[H]

	Params Parameters
}

// NewStore constructs a Store over datastore.
// The datastore must have a head there otherwise Start will error.
// For first initialization of Store use NewStoreWithHead.
func NewStore[H header.Header](ds datastore.Batching, opts ...Option) (*Store[H], error) {
	return newStore[H](ds, opts...)
}

// NewStoreWithHead initiates a new Store and forcefully sets a given trusted header as head.
func NewStoreWithHead[H header.Header](
	ctx context.Context,
	ds datastore.Batching,
	head H,
	opts ...Option,
) (*Store[H], error) {
	store, err := newStore[H](ds, opts...)
	if err != nil {
		return nil, err
	}

	return store, store.Init(ctx, head)
}

func newStore[H header.Header](ds datastore.Batching, opts ...Option) (*Store[H], error) {
	params := DefaultParameters()
	for _, opt := range opts {
		opt(&params)
	}

	if err := params.Validate(); err != nil {
		return nil, fmt.Errorf("header/store: store creation failed: %w", err)
	}

	cache, err := lru.NewARC(params.StoreCacheSize)
	if err != nil {
		return nil, fmt.Errorf("failed to create index cache: %w", err)
	}

	wrappedStore := namespace.Wrap(ds, storePrefix)
	index, err := newHeightIndexer[H](wrappedStore, params.IndexCacheSize)
	if err != nil {
		return nil, fmt.Errorf("failed to create height indexer: %w", err)
	}

	return &Store[H]{
		Params:      params,
		ds:          wrappedStore,
		heightSub:   newHeightSub[H](),
		writes:      make(chan []H, 16),
		writesDn:    make(chan struct{}),
		cache:       cache,
		heightIndex: index,
		pending:     newBatch[H](params.WriteBatchSize),
	}, nil
}

func (s *Store[H]) Init(ctx context.Context, initial H) error {
	// trust the given header as the initial head
	err := s.flush(ctx, initial)
	if err != nil {
		return err
	}

	log.Infow("initialized head", "height", initial.Height(), "hash", initial.Hash())
	return nil
}

func (s *Store[H]) Start(context.Context) error {
	go s.flushLoop()
	return nil
}

func (s *Store[H]) Stop(ctx context.Context) error {
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

func (s *Store[H]) Height() uint64 {
	return s.heightSub.Height()
}

func (s *Store[H]) Head(ctx context.Context) (H, error) {
	head, err := s.GetByHeight(ctx, s.heightSub.Height())
	if err == nil {
		return head, nil
	}

	var zero H
	head, err = s.readHead(ctx)
	switch err {
	default:
		return zero, err
	case datastore.ErrNotFound, header.ErrNotFound:
		return zero, header.ErrNoHead
	case nil:
		s.heightSub.SetHeight(uint64(head.Height()))
		log.Infow("loaded head", "height", head.Height(), "hash", head.Hash())
		return head, nil
	}
}

func (s *Store[H]) Get(ctx context.Context, hash header.Hash) (H, error) {
	var zero H
	if v, ok := s.cache.Get(hash.String()); ok {
		return v.(H), nil
	}
	// check if the requested header is not yet written on disk
	if h := s.pending.Get(hash); !h.IsZero() {
		return h, nil
	}

	b, err := s.ds.Get(ctx, datastore.NewKey(hash.String()))
	if err != nil {
		if err == datastore.ErrNotFound {
			return zero, header.ErrNotFound
		}

		return zero, err
	}

	var empty H
	h := empty.New()
	err = h.UnmarshalBinary(b)
	if err != nil {
		return zero, err
	}

	s.cache.Add(h.Hash().String(), h)
	return h.(H), nil
}

func (s *Store[H]) GetByHeight(ctx context.Context, height uint64) (H, error) {
	var zero H
	if height == 0 {
		return zero, fmt.Errorf("header/store: height must be bigger than zero")
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
	if h := s.pending.GetByHeight(height); !h.IsZero() {
		return h, nil
	}

	hash, err := s.heightIndex.HashByHeight(ctx, height)
	if err != nil {
		if err == datastore.ErrNotFound {
			return zero, header.ErrNotFound
		}

		return zero, err
	}

	return s.Get(ctx, hash)
}

func (s *Store[H]) GetRangeByHeight(ctx context.Context, from, to uint64) ([]H, error) {
	h, err := s.GetByHeight(ctx, to-1)
	if err != nil {
		return nil, err
	}

	ln := to - from
	headers := make([]H, ln)
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

func (s *Store[H]) GetVerifiedRange(
	ctx context.Context,
	from H,
	to uint64,
) ([]H, error) {
	if uint64(from.Height()) >= to {
		return nil, fmt.Errorf("header/store: invalid range(%d,%d)", from.Height(), to)
	}
	headers, err := s.GetRangeByHeight(ctx, uint64(from.Height())+1, to)
	if err != nil {
		return nil, err
	}

	for _, h := range headers {
		err := from.Verify(h)
		if err != nil {
			return nil, err
		}
		from = h
	}
	return headers, nil
}

func (s *Store[H]) Has(ctx context.Context, hash header.Hash) (bool, error) {
	if ok := s.cache.Contains(hash.String()); ok {
		return ok, nil
	}
	// check if the requested header is not yet written on disk
	if ok := s.pending.Has(hash); ok {
		return ok, nil
	}

	return s.ds.Has(ctx, datastore.NewKey(hash.String()))
}

func (s *Store[H]) HasAt(_ context.Context, height uint64) bool {
	return height != uint64(0) && s.Height() >= height
}

func (s *Store[H]) Append(ctx context.Context, headers ...H) error {
	lh := len(headers)
	if lh == 0 {
		return nil
	}

	var err error
	// take current write head to verify headers against
	var head H
	headPtr := s.writeHead.Load()
	if headPtr == nil {
		head, err = s.Head(ctx)
		if err != nil {
			return err
		}
	} else {
		head = *headPtr
	}

	// collect valid headers
	verified := make([]H, 0, lh)
	for i, h := range headers {
		// currently store requires all headers to be appended sequentially and adjacently
		// TODO(@Wondertan): Further pruning friendly Store design should reevaluate this requirement
		if h.Height() != head.Height()+1 {
			return &header.ErrNonAdjacent{
				Head:      head.Height(),
				Attempted: h.Height(),
			}
		}

		err = head.Verify(h)
		if err != nil {
			var verErr *header.VerifyError
			if errors.As(err, &verErr) {
				log.Errorw("invalid header",
					"height_of_head", head.Height(),
					"hash_of_head", head.Hash(),
					"height_of_invalid", h.Height(),
					"hash_of_invalid", h.Hash(),
					"reason", verErr.Reason)
			}
			// if the first header is invalid, no need to go further
			if i == 0 {
				// and simply return
				return err
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
		s.writeHead.Store(&verified[ln-1])
		wh := *s.writeHead.Load()
		log.Infow("new head", "height", wh.Height(), "hash", wh.Hash())
		// we return an error here after writing,
		// as there might be an invalid header in between of a given range
		return err
	case <-s.writesDn:
		return errStoppedStore
	case <-ctx.Done():
		return ctx.Err()
	}
}

// flushLoop performs writing task to the underlying datastore in a separate routine
// This way writes are controlled and manageable from one place allowing
// (1) Appends not to be blocked on long disk IO writes and underlying DB compactions
// (2) Batching header writes
func (s *Store[H]) flushLoop() {
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
		if s.pending.Len() < s.Params.WriteBatchSize && headers != nil {
			continue
		}

		err := s.flush(ctx, s.pending.GetAll()...)
		if err != nil {
			// TODO(@Wondertan): Should this be a fatal error case with os.Exit?
			from, to := uint64(headers[0].Height()), uint64(headers[len(headers)-1].Height())
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
func (s *Store[H]) flush(ctx context.Context, headers ...H) error {
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
func (s *Store[H]) readHead(ctx context.Context) (H, error) {
	var zero H
	b, err := s.ds.Get(ctx, headKey)
	if err != nil {
		return zero, err
	}

	var head header.Hash
	err = head.UnmarshalJSON(b)
	if err != nil {
		return zero, err
	}

	return s.Get(ctx, head)
}
