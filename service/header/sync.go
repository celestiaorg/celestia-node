package header

import (
	"context"
	"sync/atomic"

	"github.com/libp2p/go-libp2p-core/peer"
	pubsub "github.com/libp2p/go-libp2p-pubsub"

	tmbytes "github.com/tendermint/tendermint/libs/bytes"
)

// Syncer implements simplest possible synchronization for headers.
type Syncer struct {
	sub *P2PSubscriber
	exchange Exchange
	store    Store
	trusted  tmbytes.HexBytes

	// inProgress is set to 1 once syncing commences and
	// is set to 0 once syncing is either finished or
	// not currently in progress
	inProgress uint64
}

// NewSyncer creates a new instance of Syncer.
func NewSyncer(exchange Exchange, store Store, sub *P2PSubscriber, trusted tmbytes.HexBytes) *Syncer {
	return &Syncer{
		sub: sub,
		exchange:   exchange,
		store:      store,
		trusted:    trusted,
		inProgress: 0, // syncing is not currently in progress
	}
}

// Start starts the syncing routine.
func (s *Syncer) Start(context.Context) error {
	if s.sub != nil {
		err := s.sub.AddValidator(s.Validate)
		if err != nil {
			return err
		}
	}

	go s.Sync(context.TODO()) // TODO @Wondertan: leaving this to you to implement in disconnection toleration PR.
	return nil
}

// Sync syncs all headers up to the latest known header in the network.
func (s *Syncer) Sync(ctx context.Context) {
	log.Info("syncing headers")
	// indicate syncing
	s.syncInProgress()
	// when method returns, toggle inProgress off
	defer s.finishSync()
	// TODO(@Wondertan): Retry logic
	for {
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

// Validate implements validation of incoming Headers and stores them if they are good.
func (s *Syncer) Validate(ctx context.Context, p peer.ID, msg *pubsub.Message) pubsub.ValidationResult {
	header, err := UnmarshalExtendedHeader(msg.Data)
	if err != nil {
		log.Errorw("unmarshalling ExtendedHeader received from the PubSub",
			"err", err, "peer", p.ShortString())
		return pubsub.ValidationReject
	}

	// if syncing is still in progress - just ignore the new header as
	// Syncer will fetch it after anyway, but if syncer is done, append
	// the header.
	if !s.IsSyncing() {
		err := s.store.Append(ctx, header)
		if err != nil {
			log.Errorw("appending store with header from PubSub",
				"hash", header.Hash().String(), "height", header.Height, "peer", p.ShortString())
			// TODO(@Wondertan): We need to be sure that the error is actually validation error.
			//  Rejecting msgs due to storage error is not good, but for now that's fine.
			return pubsub.ValidationReject
		}

		// we are good to go
		return pubsub.ValidationAccept
	}

	// TODO(@Wondertan): For now we just reject incoming headers if we are not yet synced.
	//  Ideally, we should keep them optimistically and verify after Sync to avoid unnecessary requests.
	//  This introduces additional complexity for which we don't have time at the given moment.
	return pubsub.ValidationIgnore
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
