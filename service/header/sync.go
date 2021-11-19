package header

import (
	"context"

	"github.com/libp2p/go-libp2p-core/peer"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	tmbytes "github.com/tendermint/tendermint/libs/bytes"
)

// Syncer implements simplest possible synchronization for headers.
type Syncer struct {
	exchange Exchange
	store    Store
	genesis  tmbytes.HexBytes

	// done is triggered when syncing is finished
	// it assumes that Syncer is only used once
	done chan struct{}
}

// NewSyncer creates a new instance of Syncer.
func NewSyncer(exchange Exchange, store Store, genesis tmbytes.HexBytes) *Syncer {
	return &Syncer{
		exchange: exchange,
		store:    store,
		genesis:  genesis,
		done:     make(chan struct{}),
	}
}

// Sync syncs all headers up to the latest known header in the network.
func (s *Syncer) Sync(ctx context.Context) {
	log.Info("syncing headers")
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
			close(s.done)
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

// Validate implements validation of incoming Headers and stores them if they are good.
func (s *Syncer) Validate(ctx context.Context, p peer.ID, msg *pubsub.Message) pubsub.ValidationResult {
	header, err := UnmarshalExtendedHeader(msg.Data)
	if err != nil {
		log.Errorw("unmarshalling ExtendedHeader received from the PubSub",
			"err", err, "peer", p.ShortString())
		return pubsub.ValidationReject
	}

	// if syncing is still in progress - just ignore the new Header
	// Syncer will fetch it after anyway
	select {
	case <-s.done:
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
	default:
	}

	// TODO(@Wondertan): For now we just reject incoming headers if we are not yet synced.
	//  Ideally, we should keep them optimistically and verify after Sync to avoid unnecessary requests.
	//  This introduces additional complexity for which we don't have time at the given moment.
	return pubsub.ValidationIgnore
}

// getHead tries to get head locally and if not exists requests genesis.
func (s *Syncer) getHead(ctx context.Context) (*ExtendedHeader, error) {
	head, err := s.store.Head(ctx)
	switch err {
	case nil:
		return head, err
	case ErrNoHead:
		// if there is no head - setup genesis.
		genesis, err := s.exchange.RequestByHash(ctx, s.genesis)
		if err != nil {
			log.Errorw("requesting genesis", "err", err)
			return nil, err
		}

		err = s.store.Append(ctx, genesis)
		if err != nil {
			log.Errorw("appending genesis to store", "err", err)
			return nil, err
		}
	}

	return head, err
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
