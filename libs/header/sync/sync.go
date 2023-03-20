package sync

import (
	"context"
	"errors"
	"sync"
	"time"

	logging "github.com/ipfs/go-log/v2"

	"github.com/celestiaorg/celestia-node/libs/header"
)

var log = logging.Logger("header/sync")

// Syncer implements efficient synchronization for headers.
//
// Subjective Head - the latest known local valid header and a sync target.
// Network Head - the latest valid network-wide header. Becomes subjective once applied locally.
//
// There are two main processes running in Syncer:
// - Main syncing loop(s.syncLoop)
//   - Performs syncing from the latest stored header up to the latest known Subjective Head
//   - Syncs by requesting missing headers from Exchange or
//   - By accessing cache of pending headers
//
// - Receives every new Network Head from PubSub gossip subnetwork (s.incomingNetworkHead)
//   - Validates against the latest known Subjective Head, is so
//   - Sets as the new Subjective Head, which
//   - if there is a gap between the previous and the new Subjective Head
//   - Triggers s.syncLoop and saves the Subjective Head in the pending so s.syncLoop can access it
type Syncer[H header.Header] struct {
	sub     header.Subscriber[H] // to subscribe for new Network Heads
	store   syncStore[H]         // to store all the headers to
	getter  syncGetter[H]        // to fetch headers from
	metrics *metrics

	// stateLk protects state which represents the current or latest sync
	stateLk sync.RWMutex
	state   State

	// signals to start syncing
	triggerSync chan struct{}
	// pending keeps ranges of valid new network headers awaiting to be appended to store
	pending ranges[H]

	// controls lifecycle for syncLoop
	ctx    context.Context
	cancel context.CancelFunc

	Params *Parameters
}

// NewSyncer creates a new instance of Syncer.
func NewSyncer[H header.Header](
	getter header.Getter[H],
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
		store:       syncStore[H]{Store: store},
		getter:      syncGetter[H]{Getter: getter},
		triggerSync: make(chan struct{}, 1), // should be buffered
		Params:      &params,
	}, nil
}

// Start starts the syncing routine.
func (s *Syncer[H]) Start(ctx context.Context) error {
	s.ctx, s.cancel = context.WithCancel(context.Background())
	// register validator for header subscriptions
	// syncer does not subscribe itself and syncs headers together with validation
	err := s.sub.AddValidator(s.incomingNetworkHead)
	if err != nil {
		return err
	}
	// gets the latest head and kicks off syncing if necessary
	_, err = s.Head(ctx)
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

	head, err := s.store.Head(s.ctx)
	if err == nil {
		state.Height = uint64(head.Height())
	} else if state.Error == nil {
		// don't ignore the error if we can show it in the state
		state.Error = err
	}

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

// sync ensures we are synced from the Store's head up to the new subjective head.
func (s *Syncer[H]) sync(ctx context.Context) {
	subjHead, err := s.subjectiveHead(ctx)
	if err != nil {
		log.Errorw("getting subjective head", "err", err)
		return
	}

	storeHead, err := s.store.Head(ctx)
	if err != nil {
		log.Errorw("getting stored head", "err", err)
		return
	}

	if storeHead.Height() >= subjHead.Height() {
		log.Warnw("sync attempt to an already synced header",
			"synced_height", storeHead.Height(),
			"attempted_height", subjHead.Height(),
		)
		log.Warn("PLEASE REPORT THIS AS A BUG")
		return // should never happen, but just in case
	}

	from := storeHead.Height() + 1
	log.Infow("syncing headers",
		"from", from,
		"to", subjHead.Height())

	err = s.doSync(ctx, storeHead, subjHead)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			// don't log this error as it is normal case of Syncer being stopped
			return
		}

		log.Errorw("syncing headers",
			"from", from,
			"to", subjHead.Height(),
			"err", err)
		return
	}

	log.Infow("finished syncing headers",
		"from", from,
		"to", subjHead.Height(),
		"elapsed time", s.state.End.Sub(s.state.Start))
}

// doSync performs actual syncing updating the internal State.
func (s *Syncer[H]) doSync(ctx context.Context, fromHead, toHead H) (err error) {
	s.stateLk.Lock()
	s.state.ID++
	s.state.FromHeight = uint64(fromHead.Height()) + 1
	s.state.ToHeight = uint64(toHead.Height())
	s.state.FromHash = fromHead.Hash()
	s.state.ToHash = toHead.Hash()
	s.state.Start = time.Now()
	s.stateLk.Unlock()

	err = s.processHeaders(ctx, fromHead, uint64(toHead.Height()))

	s.stateLk.Lock()
	s.state.End = time.Now()
	s.state.Error = err
	s.stateLk.Unlock()
	return err
}

// processHeaders fetches and stores asked headers (from:to].
// Checks headers in pending cache that apply to the requested range.
// If some headers are missing, it starts requesting them from the network.
func (s *Syncer[H]) processHeaders(
	ctx context.Context,
	fromHead H,
	to uint64,
) (err error) {
	for {
		headersRange, ok := s.pending.First()
		if !ok {
			break
		}

		headers := headersRange.Get(to)
		if len(headers) == 0 {
			break
		}

		// check if returned range is not adjacent to `fromHead`
		if fromHead.Height()+1 != headers[0].Height() {
			// if so - request missing ones
			to := uint64(headers[0].Height() - 1)
			if err = s.requestHeaders(ctx, fromHead, to); err != nil {
				return err
			}
		}

		// apply cached headers
		if err = s.storeHeaders(ctx, headers...); err != nil {
			return err
		}

		// cleanup range only after we stored the headers
		headersRange.Remove(to)
		// update fromHead for the next iteration
		fromHead = headers[len(headers)-1]
	}
	return s.requestHeaders(ctx, fromHead, to)
}

// requestHeaders requests headers from the network -> (fromHeader.Height : to].
func (s *Syncer[H]) requestHeaders(
	ctx context.Context,
	fromHead H,
	to uint64,
) error {
	amount := to - uint64(fromHead.Height())
	// start requesting headers until amount remaining will be 0
	for amount > 0 {
		size := header.MaxRangeRequestSize
		if amount < size {
			size = amount
		}

		headers, err := s.getter.GetVerifiedRange(ctx, fromHead, size)
		if err != nil {
			return err
		}

		if err := s.storeHeaders(ctx, headers...); err != nil {
			return err
		}

		amount -= size // size == len(headers)
		fromHead = headers[len(headers)-1]
	}
	return nil
}

// storeHeaders updates store with new headers and updates current syncStore's Head.
func (s *Syncer[H]) storeHeaders(ctx context.Context, headers ...H) error {
	// we don't expect any issues in storing right now, as all headers are now verified.
	// So, we should return immediately in case an error appears.
	err := s.store.Append(ctx, headers...)
	if err != nil {
		return err
	}

	s.metrics.recordTotalSynced(len(headers))
	return nil
}
