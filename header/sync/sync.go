package sync

import (
	"context"
	"errors"
	"sync"
	"time"

	logging "github.com/ipfs/go-log/v2"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	tmbytes "github.com/tendermint/tendermint/libs/bytes"

	"github.com/celestiaorg/celestia-node/header"
)

var log = logging.Logger("header/sync")

// Syncer implements efficient synchronization for headers.
//
// There are two main processes running in Syncer:
// 1. Main syncing loop(s.syncLoop)
//    * Performs syncing from the subjective(local chain view) header up to the latest known subjective header
//    * Syncs by requesting missing headers from Exchange or
//    * By accessing cache of pending and verified headers
// 2. Receives new headers from PubSub subnetwork (s.incomingHead)
//    * Usually, a new header is adjacent to the subjective head and if so, it is simply appended to the local store,
//    incrementing the subjective height and making it the new latest known subjective header.
//    * Or, if it receives a header further in the future,
//      * verifies against the latest known subjective header
//    	* adds the header to pending cache(making it the latest known subjective header)
//      * and triggers syncing loop to catch up to that point.
type Syncer struct {
	sub      header.Subscriber
	exchange header.Exchange
	store    header.Store

	// blockTime provides a reference point for the Syncer to determine
	// whether its subjective head is outdated
	blockTime time.Duration

	// stateLk protects state which represents the current or latest sync
	stateLk sync.RWMutex
	state   State
	// signals to start syncing
	triggerSync chan struct{}
	// pending keeps ranges of valid headers received from the network awaiting to be appended to store
	pending ranges
	// cancel cancels syncLoop's context
	cancel context.CancelFunc
}

// NewSyncer creates a new instance of Syncer.
func NewSyncer(exchange header.Exchange, store header.Store, sub header.Subscriber, blockTime time.Duration) *Syncer {
	return &Syncer{
		sub:         sub,
		exchange:    exchange,
		store:       store,
		blockTime:   blockTime,
		triggerSync: make(chan struct{}, 1), // should be buffered
	}
}

// Start starts the syncing routine.
func (s *Syncer) Start(context.Context) error {
	err := s.sub.AddValidator(s.incomingHead)
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
func (s *Syncer) Stop(ctx context.Context) error {
	s.cancel()
	return s.sub.Stop(ctx)
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

// Head tries to return the Syncer's view of the objective head of the
// network.
func (s *Syncer) Head(ctx context.Context) (*header.ExtendedHeader, error) {
	return s.objectiveHead(ctx)
}

// State collects all the information about a sync.
type State struct {
	ID                   uint64 // incrementing ID of a sync
	Height               uint64 // height at the moment when State is requested for a sync
	FromHeight, ToHeight uint64 // the starting and the ending point of a sync
	FromHash, ToHash     tmbytes.HexBytes
	Start, End           time.Time
	Error                error // the error that might happen within a sync
}

// Finished returns true if sync is done, false otherwise.
func (s State) Finished() bool {
	return s.ToHeight <= s.Height
}

// Duration returns the duration of the sync.
func (s State) Duration() time.Duration {
	return s.End.Sub(s.Start)
}

// State reports state of the current (if in progress), or last sync (if finished).
// Note that throughout the whole Syncer lifetime there might an initial sync and multiple catch-ups.
// All of them are treated as different syncs with different state IDs and other information.
func (s *Syncer) State() State {
	s.stateLk.RLock()
	state := s.state
	s.stateLk.RUnlock()
	state.Height = s.store.Height()
	return state
}

// subjectiveHead returns the latest known local subjective header
// Lazily performs 'automatic subjective initialization' in case the header is expired
func (s *Syncer) subjectiveHead(ctx context.Context) (*header.ExtendedHeader, error) {
	pendHead := s.pending.Head()
	if pendHead != nil {
		return pendHead, nil
	}

	subjHead, err := s.store.Head(ctx)
	if err != nil {
		return nil, err
	}

	if !subjHead.IsExpired() {
		return subjHead, nil
	}

	// if expired - request head from a trusted peer
	trustedHead, err := s.exchange.Head(ctx)
	if err != nil {
		return nil, err
	}

	// set trusted head from trusted peer as new head without verification
	// in other words do 'subjective initialization'
	s.newHead(subjHead, trustedHead, false)
	return trustedHead, nil
}

// objectiveHead returns the latest objective network header
func (s *Syncer) objectiveHead(ctx context.Context) (*header.ExtendedHeader, error) {
	sbjHead, err := s.subjectiveHead(ctx)
	if err != nil {
		return nil, err
	}

	// if subjective header is recent enough (relative to the network's block time) - just use it
	if sbjHead.IsRecent(s.blockTime) {
		return sbjHead, nil
	}

	// TODO: Ensure only one request is happening at the time

	// otherwise, request head from a trustedPeer
	objHead, err := s.exchange.Head(ctx)
	if err != nil {
		return nil, err
	}

	// verify head returned from trustedPeer against subjective head
	res := s.newHead(sbjHead, objHead, true)
	if res == pubsub.ValidationAccept {
		// return new head
		return objHead, nil
	}
	// objHead was either ignored or rejected, return most current known
	// header to the Syncer
	return sbjHead, nil
}

// incomingHead processes new incomingHead Headers, validates them and stores/caches if applicable.a trustedPeer
func (s *Syncer) incomingHead(ctx context.Context, maybeHead *header.ExtendedHeader) pubsub.ValidationResult {
	// 1. Try to append. If header is not adjacent/from future - try it for pending cache below
	_, err := s.store.Append(ctx, maybeHead)
	switch err {
	case nil:
		// a happy case where we append adjacent header correctly
		return pubsub.ValidationAccept
	case header.ErrNonAdjacent:
		// not adjacent, so try to cache it after verifying
	default:
		var verErr *header.VerifyError
		if errors.As(err, &verErr) {
			return pubsub.ValidationReject
		}

		log.Errorw("appending header",
			"height", maybeHead.Height,
			"hash", maybeHead.Hash().String(),
			"err", err)
		// might be a storage error or something else, but we can still try to continue processing 'maybeHead'
	}

	// 2. Get known subjective head, so we can verify maybeHead
	subjHead, err := s.subjectiveHead(ctx)
	if err != nil {
		log.Errorw("getting subjective head", "err", err)
		return pubsub.ValidationIgnore // we don't know if header is invalid so ignore
	}

	return s.newHead(subjHead, maybeHead, true)
}

// newHead verifies the potential new head against the Syncer's subjective head, and if
// the potential new head is valid, sets it as the sync target.
func (s *Syncer) newHead(subjective, new *header.ExtendedHeader, verify bool) pubsub.ValidationResult {
	if verify {
		if subjective.Equals(new) {
			return pubsub.ValidationAccept
		}

		// 1. Filter out maybe if behind known
		if !subjective.IsBefore(new) {
			log.Warnw("received known header",
				"height", new.Height,
				"hash", new.Hash())
			return pubsub.ValidationIgnore // we don't know if header is invalid so ignore
		}

		// 2. Verify maybeHead against subjective
		err := subjective.VerifyNonAdjacent(new)
		var verErr *header.VerifyError
		if errors.As(err, &verErr) {
			log.Errorw("invalid header",
				"height_of_invalid", new.Height,
				"hash_of_invalid", new.Hash(),
				"height_of_subjective", subjective.Height,
				"hash_of_subjective", subjective.Hash(),
				"reason", verErr.Reason)
			return pubsub.ValidationReject
		}
	}

	s.wantSync()
	s.pending.Add(new)
	log.Infow("pending head", "height", new.Height, "hash", new.Hash())
	return pubsub.ValidationAccept
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

// sync ensures we are synced up to any verified header.
func (s *Syncer) sync(ctx context.Context) {
	objHead, err := s.objectiveHead(ctx)
	if err != nil {
		log.Errorw("getting objective head", "err", err)
		return
	}

	s.syncTo(ctx, objHead)
}

// syncTo requests headers from locally stored head up to the new head.
func (s *Syncer) syncTo(ctx context.Context, newHead *header.ExtendedHeader) {
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

// doSync performs actual syncing updating the internal State
func (s *Syncer) doSync(ctx context.Context, fromHead, toHead *header.ExtendedHeader) (err error) {
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
func (s *Syncer) findHeaders(ctx context.Context, from, to uint64) ([]*header.ExtendedHeader, error) {
	amount := to - from + 1 // + 1 to include 'to' height as well
	if amount > requestSize {
		to, amount = from+requestSize, requestSize
	}

	out := make([]*header.ExtendedHeader, 0, amount)
	for from < to {
		// if we have some range cached - use it
		r, ok := s.pending.FirstRangeWithin(from, to)
		if !ok {
			hs, err := s.exchange.GetRangeByHeight(ctx, from, amount)
			return append(out, hs...), err
		}

		// first, request everything between from and start of the found range
		hs, err := s.exchange.GetRangeByHeight(ctx, from, r.start-from)
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
