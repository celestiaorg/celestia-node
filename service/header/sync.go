package header

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	tmbytes "github.com/tendermint/tendermint/libs/bytes"
)

// Syncer implements efficient synchronization for headers.
//
// There are two main processes running in Syncer:
// 1. Main syncing loop(s.syncLoop)
//    * Performs syncing from the subjective(local chain view) header up to the latest known trusted header
//    * Syncs by requesting missing headers from Exchange or
//    * By accessing cache of pending and verified headers
// 2. Receives new headers from PubSub subnetwork (s.processIncoming)
//    * Usually, a new header is adjacent to the trusted head and if so, it is simply appended to the local store,
//    incrementing the subjective height and making it the new latest known trusted header.
//    * Or, if it receives a header further in the future,
//      * verifies against the latest known trusted header
//    	* adds the header to pending cache(making it the latest known trusted header)
//      * and triggers syncing loop to catch up to that point.
type Syncer struct {
	sub      Subscriber
	exchange Exchange
	store    Store

	// stateLk protects state which represents the current or latest sync
	stateLk sync.RWMutex
	state   SyncState
	// signals to start syncing
	triggerSync chan struct{}
	// pending keeps ranges of valid headers received from the network awaiting to be appended to store
	pending ranges
	// cancel cancels syncLoop's context
	cancel context.CancelFunc
}

// NewSyncer creates a new instance of Syncer.
func NewSyncer(exchange Exchange, store Store, sub Subscriber) *Syncer {
	return &Syncer{
		sub:         sub,
		exchange:    exchange,
		store:       store,
		triggerSync: make(chan struct{}, 1), // should be buffered
	}
}

// Start starts the syncing routine.
func (s *Syncer) Start(context.Context) error {
	if s.cancel != nil {
		return fmt.Errorf("header: Syncer already started")
	}

	err := s.sub.AddValidator(s.processIncoming)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	go s.syncLoop(ctx)
	s.wantSync()
	s.cancel = cancel
	return nil
}

// Stop stops Syncer.
func (s *Syncer) Stop(context.Context) error {
	s.cancel()
	s.cancel = nil
	return nil
}

// WaitSync blocks until ongoing sync is done.
func (s *Syncer) WaitSync(ctx context.Context) error {
	state := s.State()
	if state.Finished() {
		return nil
	}

	// this store method blocks until header is available
	_, err := s.store.GetByHeight(ctx, state.ToHeight)
	return err
}

// SyncState collects all the information about a sync.
type SyncState struct {
	ID                   uint64 // incrementing ID of a sync
	Height               uint64 // height at the moment when State is requested for a sync
	FromHeight, ToHeight uint64 // the starting and the ending point of a sync
	FromHash, ToHash     tmbytes.HexBytes
	Start, End           time.Time
	Error                error // the error that might happen within a sync
}

// Finished returns true if sync is done, false otherwise.
func (s SyncState) Finished() bool {
	return s.ToHeight <= s.Height
}

// State reports state of the current (if in progress), or last sync (if finished).
// Note that throughout the whole Syncer lifetime there might an initial sync and multiple catch-ups.
// All of them are treated as different syncs with different state IDs and other information.
func (s *Syncer) State() SyncState {
	s.stateLk.RLock()
	state := s.state
	s.stateLk.RUnlock()
	state.Height = s.store.Height()
	return state
}

// trustedHead returns the latest known trusted header that is within the trusting period.
func (s *Syncer) trustedHead(ctx context.Context) (*ExtendedHeader, error) {
	// check pending for trusted header and return it if applicable
	// NOTE: Pending cannot be expired, guaranteed
	pendHead := s.pending.Head()
	if pendHead != nil {
		return pendHead, nil
	}

	sbj, err := s.store.Head(ctx)
	if err != nil {
		return nil, err
	}

	// check if our subjective header is not expired and use it
	if !sbj.IsExpired() {
		return sbj, nil
	}

	// otherwise, request head from a trustedPeer or, in other words, do automatic subjective initialization
	objHead, err := s.exchange.RequestHead(ctx)
	if err != nil {
		return nil, err
	}

	s.pending.Add(objHead)
	return objHead, nil
}

// wantSync will trigger the syncing loop (non-blocking).
func (s *Syncer) wantSync() {
	select {
	case s.triggerSync <- struct{}{}:
	default:
	}
}

// syncLoop controls syncing process.
func (s *Syncer) syncLoop(ctx context.Context) {
	for {
		select {
		case <-s.triggerSync:
			s.sync(ctx)
		case <-ctx.Done():
			return
		}
	}
}

// sync ensures we are synced up to any trusted header.
func (s *Syncer) sync(ctx context.Context) {
	trstHead, err := s.trustedHead(ctx)
	if err != nil {
		log.Errorw("getting trusted head", "err", err)
		return
	}

	s.syncTo(ctx, trstHead)
}

// processIncoming processes new processIncoming Headers, validates them and stores/caches if applicable.
func (s *Syncer) processIncoming(ctx context.Context, maybeHead *ExtendedHeader) pubsub.ValidationResult {
	// 1. Try to append. If header is not adjacent/from future - try it for pending cache below
	_, err := s.store.Append(ctx, maybeHead)
	switch err {
	case nil:
		// a happy case where we append adjacent header correctly
		return pubsub.ValidationAccept
	case ErrNonAdjacent:
		// not adjacent, so try to cache it after verifying
	default:
		var verErr *VerifyError
		if errors.As(err, &verErr) {
			return pubsub.ValidationReject
		}

		log.Errorw("appending header",
			"height", maybeHead.Height,
			"hash", maybeHead.Hash().String(),
			"err", err)
		// might be a storage error or something else, but we can still try to continue processing 'maybeHead'
	}

	// 2. Get known trusted head, so we can verify maybeHead
	trstHead, err := s.trustedHead(ctx)
	if err != nil {
		log.Errorw("getting trusted head", "err", err)
		return pubsub.ValidationIgnore // we don't know if header is invalid so ignore
	}

	// 3. Filter out maybeHead if behind trusted
	if maybeHead.Height <= trstHead.Height {
		log.Warnw("received known header",
			"height", maybeHead.Height,
			"hash", maybeHead.Hash())
		return pubsub.ValidationIgnore // we don't know if header is invalid so ignore
	}

	// 4. Verify maybeHead against trusted
	err = trstHead.VerifyNonAdjacent(maybeHead)
	var verErr *VerifyError
	if errors.As(err, &verErr) {
		log.Errorw("invalid header",
			"height_of_invalid", maybeHead.Height,
			"hash_of_invalid", maybeHead.Hash(),
			"height_of_trusted", trstHead.Height,
			"hash_of_trusted", trstHead.Hash(),
			"reason", verErr.Reason)
		return pubsub.ValidationReject
	}

	// 5. Save verified header to pending cache
	// NOTE: Pending cache can't be DOSed as we verify above each header against a trusted one.
	s.pending.Add(maybeHead)
	// and trigger sync to catch-up
	s.wantSync()
	log.Infow("pending head",
		"height", maybeHead.Height,
		"hash", maybeHead.Hash())
	return pubsub.ValidationAccept
}

// syncTo requests headers from locally stored head up to the new head.
func (s *Syncer) syncTo(ctx context.Context, newHead *ExtendedHeader) {
	head, err := s.store.Head(ctx)
	if err != nil {
		log.Errorw("getting head during sync", "err", err)
		return
	}

	if head.Height == newHead.Height {
		return
	}

	log.Infow("syncing headers",
		"from", head.Height,
		"to", newHead.Height)
	err = s.doSync(ctx, head, newHead)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			// don't log this error as it is normal case of Syncer being stopped
			return
		}

		log.Errorw("syncing headers",
			"from", head.Height,
			"to", newHead.Height,
			"err", err)
		return
	}

	log.Infow("finished syncing",
		"from", head.Height,
		"to", newHead.Height,
		"elapsed time", s.state.End.Sub(s.state.Start))
}

// doSync performs actual syncing updating the internal SyncState
func (s *Syncer) doSync(ctx context.Context, fromHead, toHead *ExtendedHeader) (err error) {
	from, to := uint64(fromHead.Height)+1, uint64(toHead.Height)

	s.stateLk.Lock()
	s.state.ID++
	s.state.FromHeight = from
	s.state.ToHeight = to
	s.state.FromHash = fromHead.Hash()
	s.state.ToHash = toHead.Hash()
	s.state.Start = time.Now()
	s.stateLk.Unlock()

	for processed := 0; from < to; from += uint64(processed) {
		processed, err = s.processHeaders(ctx, from, to)
		if err != nil && processed == 0 {
			break
		}
	}

	s.stateLk.Lock()
	s.state.End = time.Now()
	s.state.Error = err
	s.stateLk.Unlock()
	return err
}

// processHeaders gets and stores headers starting at the given 'from' height up to 'to' height - [from:to]
func (s *Syncer) processHeaders(ctx context.Context, from, to uint64) (int, error) {
	headers, err := s.findHeaders(ctx, from, to)
	if err != nil {
		return 0, err
	}

	return s.store.Append(ctx, headers...)
}

// TODO(@Wondertan): Number of headers that can be requested at once. Either make this configurable or,
//  find a proper rationale for constant.
// TODO(@Wondertan): Make configurable
var requestSize uint64 = 512

// findHeaders gets headers from either remote peers or from local cache of headers received by PubSub - [from:to]
func (s *Syncer) findHeaders(ctx context.Context, from, to uint64) ([]*ExtendedHeader, error) {
	amount := to - from + 1 // + 1 to include 'to' height as well
	if amount > requestSize {
		to, amount = from+requestSize, requestSize
	}

	out := make([]*ExtendedHeader, 0, amount)
	for from < to {
		// if we have some range cached - use it
		r, ok := s.pending.FirstRangeWithin(from, to)
		if !ok {
			hs, err := s.exchange.RequestHeaders(ctx, from, amount)
			return append(out, hs...), err
		}

		// first, request everything between from and start of the found range
		hs, err := s.exchange.RequestHeaders(ctx, from, r.Start-from)
		if err != nil {
			return nil, err
		}
		out = append(out, hs...)
		from += uint64(len(hs))

		// then, apply cached range if any
		cached, ln := r.Before(to)
		out = append(out, cached...)
		from += ln
	}

	return out, nil
}
