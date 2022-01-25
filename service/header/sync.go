package header

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
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
	// inProgress is set to 1 once syncing commences and
	// is set to 0 once syncing is either finished or
	// not currently in progress
	inProgress uint64
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

// IsSyncing returns the current sync status of the Syncer.
func (s *Syncer) IsSyncing() bool {
	return atomic.LoadUint64(&s.inProgress) == 1
}

// SyncState collects all the information about o sync.
type SyncState struct {
	ID                           uint64
	FromHeight, ToHeight, Height uint64
	FromHash, ToHash             tmbytes.HexBytes
	Start, End                   time.Time
	Error                        error
}

// State reports state of current, if in progress, or last sync, if finished.
func (s *Syncer) State() SyncState {
	s.stateLk.RLock()
	defer s.stateLk.RUnlock()
	return s.state
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
	// indicate syncing
	atomic.StoreUint64(&s.inProgress, 1)
	// indicate syncing is stopped
	defer atomic.StoreUint64(&s.inProgress, 0)

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
			log.Errorw("invalid header",
				"height", maybeHead.Height,
				"hash", maybeHead.Hash(),
				"reason", verErr.Reason)
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
			"height", maybeHead.Height,
			"hash", maybeHead.Hash(),
			"reason", verErr.Reason)
		return pubsub.ValidationReject
	}

	// 5. Save verified header to pending cache
	// NOTE: Pending cache can't be DOSed as we verify above each header against a trusted one.
	s.pending.Add(maybeHead)
	// and trigger sync to catch-up
	s.wantSync()
	log.Infow("new pending head",
		"height", maybeHead.Height,
		"hash", maybeHead.Hash())
	return pubsub.ValidationAccept
}

// TODO(@Wondertan): Number of headers that can be requested at once. Either make this configurable or,
//  find a proper rationale for constant.
var requestSize uint64 = 512

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
		log.Errorw("syncing headers",
			"from", head.Height,
			"to", newHead.Height,
			"err", err)
		return
	}

	log.Info("synced headers",
		"from", head.Height,
		"to", newHead.Height,
		"took", s.state.End.Sub(s.state.Start))
}

// doSync performs actual syncing updating the internal SyncState
func (s *Syncer) doSync(ctx context.Context, oldHead, newHead *ExtendedHeader) (err error) {
	from, to := uint64(oldHead.Height)+1, uint64(newHead.Height)

	s.stateLk.Lock()
	s.state.ID++
	s.state.FromHeight = from
	s.state.ToHeight = to
	s.state.FromHash = oldHead.Hash()
	s.state.ToHash = newHead.Hash()
	s.state.Start = time.Now()
	s.stateLk.Unlock()

	for from <= to {
		amount := to - from + 1
		if amount > requestSize {
			amount = requestSize
		}

		amount, err = s.processHeaders(ctx, from, amount)
		if err != nil && amount == 0 {
			break
		}

		from += amount
		s.stateLk.Lock()
		s.state.Height = from
		s.stateLk.Unlock()
	}

	s.stateLk.Lock()
	s.state.Height = from - 1 // minus one as we add one to the amount above
	s.state.End = time.Now()
	s.state.Error = err
	s.stateLk.Unlock()
	return err
}

// processHeaders gets and stores the 'amount' number of headers going from the 'start' height.
func (s *Syncer) processHeaders(ctx context.Context, from, amount uint64) (uint64, error) {
	headers, err := s.getHeaders(ctx, from, amount)
	if err != nil {
		return 0, err
	}

	ln, err := s.store.Append(ctx, headers...)
	if err != nil {
		return 0, err
	}

	return uint64(ln), nil
}

// getHeaders gets headers from either remote peers or from local cache of headers received by PubSub
func (s *Syncer) getHeaders(ctx context.Context, start, amount uint64) ([]*ExtendedHeader, error) {
	// short-circuit if nothing in pending cache to avoid unnecessary allocation below
	if _, ok := s.pending.FirstRangeWithin(start, start+amount); !ok {
		return s.exchange.RequestHeaders(ctx, start, amount)
	}

	end, out := start+amount, make([]*ExtendedHeader, 0, amount)
	for start < end {
		// if we have some range cached - use it
		if r, ok := s.pending.FirstRangeWithin(start, end); ok {
			// first, request everything between start and found range
			hs, err := s.exchange.RequestHeaders(ctx, start, r.Start-start)
			if err != nil {
				return nil, err
			}
			out = append(out, hs...)
			start += uint64(len(hs))

			// then, apply cached range
			cached := r.Before(end)
			out = append(out, cached...)
			start += uint64(len(cached))

			// repeat, as there might be multiple cache ranges
			continue
		}

		// fetch the leftovers
		hs, err := s.exchange.RequestHeaders(ctx, start, end-start)
		if err != nil {
			// still return what was successfully gotten
			return out, err
		}

		return append(out, hs...), nil
	}

	return out, nil
}
