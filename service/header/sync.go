package header

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/libp2p/go-libp2p-core/peer"
	pubsub "github.com/libp2p/go-libp2p-pubsub"

	tmbytes "github.com/tendermint/tendermint/libs/bytes"
)

// Syncer implements simplest possible synchronization for headers.
type Syncer struct {
	sub      *P2PSubscriber
	exchange Exchange
	store    Store
	trusted  tmbytes.HexBytes

	// inProgress is set to 1 once syncing commences and
	// is set to 0 once syncing is either finished or
	// not currently in progress
	inProgress  uint64
	triggerSync chan struct{}

	// pending keeps valid headers rcvd from the network awaiting to be appended to store
	pending   map[uint64]*ExtendedHeader
	pendingLk sync.Mutex

	ctx    context.Context
	cancel context.CancelFunc
}

// NewSyncer creates a new instance of Syncer.
func NewSyncer(exchange Exchange, store Store, sub *P2PSubscriber, trusted tmbytes.HexBytes) *Syncer {
	return &Syncer{
		sub:         sub,
		exchange:    exchange,
		store:       store,
		trusted:     trusted,
		triggerSync: make(chan struct{}), // no buffer needed
		pending:     make(map[uint64]*ExtendedHeader),
	}
}

// Start starts the syncing routine.
func (s *Syncer) Start(context.Context) error {
	if s.sub != nil {
		err := s.sub.AddValidator(s.validate)
		if err != nil {
			return err
		}
	}

	s.ctx, s.cancel = context.WithCancel(context.Background())
	go s.syncLoop()
	s.wantSync()
	return nil
}

func (s *Syncer) wantSync() {
	select {
	case s.triggerSync <- struct{}{}:
	}
}

func (s *Syncer) syncLoop() {
	for {
		select {
		case <-s.triggerSync:
			s.sync(s.ctx)
		case <-s.ctx.Done():
			return
		}
	}
}

// Sync syncs all headers up to the latest known header in the network.
func (s *Syncer) sync(ctx context.Context) {
	log.Info("syncing headers")
	// indicate syncing
	s.syncInProgress()
	// when method returns, toggle inProgress off
	defer s.finishSync()
	// TODO(@Wondertan): Retry logic
	for {
		if ctx.Err() != nil {
			return
		}

		localHead, err := s.getHead(ctx)
		if err != nil {
			log.Errorw("getting head", "err", err)
			return
		}

		netHead, err := s.exchange.RequestHead(ctx)
		if err != nil {
			log.Errorw("requesting network head", "err", err)
			return
		}

		if localHead.Height >= netHead.Height {
			// we are now synced
			log.Info("synced headers")
			return
		}

		err = s.syncDiff(ctx, localHead, netHead)
		if err != nil {
			log.Errorw("syncing headers", "err", err)
			return
		}
	}
}

// IsSyncing returns the current sync status of the Syncer.
func (s *Syncer) IsSyncing() bool {
	return atomic.LoadUint64(&s.inProgress) == 1
}

// syncInProgress indicates Syncer's sync status is in progress.
func (s *Syncer) syncInProgress() {
	atomic.StoreUint64(&s.inProgress, 1)
}

// finishSync indicates Syncer's sync status as no longer in progress.
func (s *Syncer) finishSync() {
	atomic.StoreUint64(&s.inProgress, 0)
}

// validate implements validation of incoming Headers and stores them if they are good.
func (s *Syncer) validate(ctx context.Context, p peer.ID, msg *pubsub.Message) pubsub.ValidationResult {
	maybeHead, err := UnmarshalExtendedHeader(msg.Data)
	if err != nil {
		log.Errorw("unmarshalling header received from the PubSub",
			"err", err, "peer", p.ShortString())
		return pubsub.ValidationReject
	}

	localHead, err := s.store.Head(ctx)
	if err != nil {
		log.Errorw("getting local head", "err", err)
		return pubsub.ValidationIgnore // we don't know if header is invalid so ignore
	}

	if maybeHead.Height <= localHead.Height {
		log.Warnw("rcvd known header", "from", p.ShortString())
		return pubsub.ValidationIgnore // we don't know if header is invalid so ignore
	}

	// validate that +2/3 of subjective validator set signed the commit and if not - reject
	err = Verify(localHead, maybeHead)
	if err != nil {
		// our subjective view can be far from objective network head(during sync), so we cannot be sure if header is
		// 100% invalid due to outdated validator set, thus ValidationIgnore and thus 'possibly malicious'
		log.Warnw("rcvd possibly malicious header",
			"hash", maybeHead.Hash(), "height", maybeHead.Height, "from", p.ShortString())
		return pubsub.ValidationIgnore
	}

	if maybeHead.Height > localHead.Height+1 {
		// we might be missing some headers, so trigger sync to catch-up
		s.wantSync()
		// and save verified header for later
		// TODO(@Wondertan): If we sync from scratch a year of headers, this may blow up memory
		s.pendingLk.Lock()
		s.pending[uint64(maybeHead.Height)] = maybeHead
		s.pendingLk.Unlock()
		// accepted
		return pubsub.ValidationAccept
	}

	// at this point we reliably know 'maybeHead' is adjacent to 'localHead' so append
	err = s.store.Append(ctx, maybeHead)
	if err != nil {
		log.Errorw("appending store with header from PubSub",
			"hash", maybeHead.Hash().String(), "height", maybeHead.Height, "peer", p.ShortString())

		// TODO(@Wondertan): We need to be sure that the error is actually validation error.
		//  Rejecting msgs due to storage error is not good, but for now that's fine.
		return pubsub.ValidationReject
	}

	// we are good to go
	return pubsub.ValidationAccept
}

// getHead tries to get head locally and if not exists requests trusted hash.
func (s *Syncer) getHead(ctx context.Context) (*ExtendedHeader, error) {
	head, err := s.store.Head(ctx)
	switch err {
	case nil:
		return head, nil
	case ErrNoHead:
		// if there is no head - request header at trusted hash.
		trusted, err := s.exchange.RequestByHash(ctx, s.trusted)
		if err != nil {
			log.Errorw("requesting header at trusted hash", "err", err)
			return nil, err
		}

		err = s.store.Append(ctx, trusted)
		if err != nil {
			log.Errorw("appending header at trusted hash to store", "err", err)
			return nil, err
		}

		return trusted, nil
	}

	return nil, err
}

// TODO(@Wondertan): Number of headers that can be requested at once. Either make this configurable or,
// find a proper rationale for constant.
var requestSize uint64 = 128

// syncDiff requests headers from knownHead up to new head.
func (s *Syncer) syncDiff(ctx context.Context, knownHead, newHead *ExtendedHeader) error {
	start, end := uint64(knownHead.Height+1), uint64(newHead.Height)
	for start < end {
		amount := end - start
		if amount > requestSize {
			amount = requestSize
		}

		headers, err := s.exchange.RequestHeaders(ctx, start, amount)
		if err != nil {
			return err
		}

		err = s.store.Append(ctx, headers...)
		if err != nil {
			return err
		}

		start += amount
	}

	return s.store.Append(ctx, newHead)
}
