package sync

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	logging "github.com/ipfs/go-log/v2"

	"github.com/celestiaorg/celestia-node/libs/header"
)

var log = logging.Logger("header/sync")

// Syncer implements efficient synchronization for headers.
//
// Subjective header - the latest known header that is not expired (within trusting period)
// Network header - the latest header received from the network
//
// There are two main processes running in Syncer:
// 1. Main syncing loop(s.syncLoop)
//   - Performs syncing from the subjective(local chain view) header up to the latest known trusted header
//   - Syncs by requesting missing headers from Exchange or
//   - By accessing cache of pending and verified headers
//
// 2. Receives new headers from PubSub subnetwork (s.processIncoming)
//   - Usually, a new header is adjacent to the trusted head and if so, it is simply appended to the local store,
//     incrementing the subjective height and making it the new latest known trusted header.
//   - Or, if it receives a header further in the future,
//     verifies against the latest known trusted header
//     adds the header to pending cache(making it the latest known trusted header)
//     and triggers syncing loop to catch up to that point.
type Syncer[H header.Header] struct {
	sub      header.Subscriber[H]
	exchange header.Exchange[H]
	store    header.Store[H]

	// stateLk protects state which represents the current or latest sync
	stateLk sync.RWMutex
	state   State
	// signals to start syncing
	triggerSync chan struct{}
	// syncedHead is the latest synced header.
	syncedHead atomic.Pointer[H]
	// pending keeps ranges of valid new network headers awaiting to be appended to store
	pending ranges[H]
	// netReqLk ensures only one network head is requested at any moment
	netReqLk sync.RWMutex

	// controls lifecycle for syncLoop
	ctx    context.Context
	cancel context.CancelFunc

	Params *Parameters

	metrics *metrics
}

// NewSyncer creates a new instance of Syncer.
func NewSyncer[H header.Header](
	exchange header.Exchange[H],
	store header.Store[H],
	sub header.Subscriber[H],
	opts ...Options,
) (*Syncer[H], error) {
	params := DefaultParameters()
	for _, opt := range opts {
		opt(&params)
	}
	if err := params.Validate(); err != nil {
		return nil, err
	}

	return &Syncer[H]{
		sub:         sub,
		exchange:    exchange,
		store:       store,
		triggerSync: make(chan struct{}, 1), // should be buffered
		Params:      &params,
	}, nil
}

// Start starts the syncing routine.
func (s *Syncer[H]) Start(ctx context.Context) error {
	s.ctx, s.cancel = context.WithCancel(context.Background())
	// register validator for header subscriptions
	// syncer does not subscribe itself and syncs headers together with validation
	err := s.sub.AddValidator(s.incomingNetHead)
	if err != nil {
		return err
	}
	// get the latest head and set it as syncing target
	_, err = s.networkHead(ctx)
	if err != nil {
		return err
	}
	// start syncLoop only if Start is errorless
	go s.syncLoop()
	return nil
}

// Stop stops Syncer.
func (s *Syncer[H]) Stop(context.Context) error {
	s.cancel()
	return nil
}

// SyncWait blocks until ongoing sync is done.
func (s *Syncer[H]) SyncWait(ctx context.Context) error {
	state := s.State()
	if state.Finished() {
		return nil
	}

	// this store method blocks until header is available
	_, err := s.store.GetByHeight(ctx, state.ToHeight)
	return err
}

// State collects all the information about a sync.
type State struct {
	ID                   uint64 // incrementing ID of a sync
	Height               uint64 // height at the moment when State is requested for a sync
	FromHeight, ToHeight uint64 // the starting and the ending point of a sync
	FromHash, ToHash     header.Hash
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
// Note that throughout the whole Syncer lifetime there might an initial sync and multiple
// catch-ups. All of them are treated as different syncs with different state IDs and other
// information.
func (s *Syncer[H]) State() State {
	s.stateLk.RLock()
	state := s.state
	s.stateLk.RUnlock()
	state.Height = s.store.Height()
	return state
}

// wantSync will trigger the syncing loop (non-blocking).
func (s *Syncer[H]) wantSync() {
	select {
	case s.triggerSync <- struct{}{}:
	default:
	}
}

// syncLoop controls syncing process.
func (s *Syncer[H]) syncLoop() {
	for {
		select {
		case <-s.triggerSync:
			s.sync(s.ctx)
		case <-s.ctx.Done():
			return
		}
	}
}

// sync ensures we are synced from the Store's head up to the new subjective head
func (s *Syncer[H]) sync(ctx context.Context) {
	newHead := s.pending.Head()
	if newHead.IsZero() {
		return
	}

	headPtr := s.syncedHead.Load()

	var header H
	if headPtr == nil {
		head, err := s.store.Head(ctx)
		if err != nil {
			log.Errorw("getting head during sync", "err", err)
			return
		}
		header = head
	} else {
		header = *headPtr
	}

	if header.Height() >= newHead.Height() {
		log.Warnw("sync attempt to an already synced header",
			"synced_height", header.Height(),
			"attempted_height", newHead.Height(),
		)
		log.Warn("PLEASE REPORT THIS AS A BUG")
		return // should never happen, but just in case
	}

	log.Infow("syncing headers",
		"from", header.Height(),
		"to", newHead.Height())
	err := s.doSync(ctx, header, newHead)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			// don't log this error as it is normal case of Syncer being stopped
			return
		}

		log.Errorw("syncing headers",
			"from", header.Height(),
			"to", newHead.Height(),
			"err", err)
		return
	}

	log.Infow("finished syncing",
		"from", header.Height(),
		"to", newHead.Height(),
		"elapsed time", s.state.End.Sub(s.state.Start))
}

// doSync performs actual syncing updating the internal State
func (s *Syncer[H]) doSync(ctx context.Context, fromHead, toHead H) (err error) {
	from, to := uint64(fromHead.Height())+1, uint64(toHead.Height())

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
		if s.metrics != nil {
			s.metrics.recordTotalSynced(processed)
		}
	}

	s.stateLk.Lock()
	s.state.End = time.Now()
	s.state.Error = err
	s.stateLk.Unlock()
	return err
}

// processHeaders gets and stores headers starting at the given 'from' height up to 'to' height -
// [from:to]
func (s *Syncer[H]) processHeaders(ctx context.Context, from, to uint64) (int, error) {
	headers, err := s.findHeaders(ctx, from, to)
	if err != nil {
		return 0, err
	}

	amount, err := s.store.Append(ctx, headers...)
	if err == nil && amount > 0 {
		s.syncedHead.Store(&headers[amount-1])
	}
	return amount, err
}

// findHeaders gets headers from either remote peers or from local cache of headers received by
// PubSub - [from:to]
func (s *Syncer[H]) findHeaders(ctx context.Context, from, to uint64) ([]H, error) {
	amount := to - from + 1 // + 1 to include 'to' height as well
	if amount > s.Params.MaxRequestSize {
		to = from + s.Params.MaxRequestSize - 1 // `from` is already included in range
		amount = s.Params.MaxRequestSize
	}

	out := make([]H, 0, amount)
	for from <= to {
		// if we have some range cached - use it
		r, ok := s.pending.FirstRangeWithin(from, to)
		if !ok {
			hs, err := s.exchange.GetRangeByHeight(ctx, from, to-from+1)
			return append(out, hs...), err
		}

		// first, request everything between from and start of the found range
		hs, err := s.exchange.GetRangeByHeight(ctx, from, r.start-from)
		if err != nil {
			return nil, err
		}
		out = append(out, hs...)
		from += uint64(len(hs))

		// apply cached range if any
		cached, ln := r.Before(to)
		out = append(out, cached...)
		from += ln
	}

	return out, nil
}
