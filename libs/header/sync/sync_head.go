package sync

import (
	"context"
	"errors"
	"time"

	pubsub "github.com/libp2p/go-libp2p-pubsub"

	"github.com/celestiaorg/celestia-node/libs/header"
)

// Head returns the Network Head.
//
// Known subjective head is considered network head if it is recent enough(now-timestamp<=blocktime)
// Otherwise, head is requested from a trusted peer and
// set as the new subjective head, assuming that trusted peer is always fully synced.
func (s *Syncer[H]) Head(ctx context.Context) (H, error) {
	sbjHead, err := s.subjectiveHead(ctx)
	if err != nil {
		return sbjHead, err
	}
	// if subjective header is recent enough (relative to the network's block time) - just use it
	if isRecent(sbjHead, s.Params.blockTime) {
		return sbjHead, nil
	}
	// otherwise, request head from a trusted peer, as we assume it is fully synced
	//
	// TODO(@Wondertan): Here is another potential networking optimization:
	//  * From sbjHead's timestamp and current time predict the time to the next header(TNH)
	//  * If now >= TNH && now <= TNH + (THP) header propagation time
	//    * Wait for header to arrive instead of requesting it
	//  * This way we don't request as we know the new network header arrives exactly
	netHead, err := s.getter.Head(ctx)
	if err != nil {
		return netHead, err
	}
	// process and validate netHead fetched from trusted peers
	// NOTE: We could trust the netHead like we do during 'automatic subjective initialization'
	// but in this case our subjective head is not expired, so we should verify netHead
	// and only if it is valid, set it as new head
	s.incomingNetworkHead(ctx, netHead)
	// netHead was either accepted or rejected as the new subjective
	// anyway return most current known subjective head
	return s.subjectiveHead(ctx)
}

// subjectiveHead returns the latest known local header that is not expired(within trusting period).
// If the header is expired, it is retrieved from a trusted peer without validation;
// in other words, an automatic subjective initialization is performed.
func (s *Syncer[H]) subjectiveHead(ctx context.Context) (H, error) {
	// pending head is the latest known subjective head and sync target, so try to get it
	// NOTES:
	// * Empty when no sync is in progress
	// * Pending cannot be expired, guaranteed
	pendHead := s.pending.Head()
	if !pendHead.IsZero() {
		return pendHead, nil
	}
	// if pending is empty - get the latest stored/synced head
	storeHead, err := s.store.Head(ctx)
	if err != nil {
		return storeHead, err
	}
	// check if the stored header is not expired and use it
	if !isExpired(storeHead, s.Params.TrustingPeriod) {
		return storeHead, nil
	}
	// otherwise, request head from a trusted peer
	log.Infow("stored head header expired", "height", storeHead.Height())
	trustHead, err := s.getter.Head(ctx)
	if err != nil {
		return trustHead, err
	}
	// and set it as the new subjective head without validation,
	// or, in other words, do 'automatic subjective initialization'
	// NOTE: we avoid validation as the head expired to prevent possibility of the Long-Range Attack
	s.setSubjectiveHead(ctx, trustHead)
	switch {
	default:
		log.Infow("subjective initialization finished", "height", trustHead.Height())
		return trustHead, nil
	case isExpired(trustHead, s.Params.TrustingPeriod):
		log.Warnw("subjective initialization with an expired header", "height", trustHead.Height())
	case !isRecent(trustHead, s.Params.blockTime):
		log.Warnw("subjective initialization with an old header", "height", trustHead.Height())
	}
	log.Warn("trusted peer is out of sync")
	return trustHead, nil
}

// setSubjectiveHead takes already validated head and sets it as the new sync target.
func (s *Syncer[H]) setSubjectiveHead(ctx context.Context, netHead H) {
	// TODO(@Wondertan): Right now, we can only store adjacent headers, instead we should:
	//  * Allow storing any valid header here in Store
	//  * Remove ErrNonAdjacent
	//  * Remove writeHead from the canonical store implementation
	err := s.storeHeaders(ctx, netHead)
	var nonAdj *header.ErrNonAdjacent
	if err != nil && !errors.As(err, &nonAdj) {
		// might be a storage error or something else, but we can still try to continue processing netHead
		log.Errorw("storing new network header",
			"height", netHead.Height(),
			"hash", netHead.Hash().String(),
			"err", err)
	}

	storeHead, err := s.store.Head(ctx)
	if err == nil && storeHead.Height() >= netHead.Height() {
		// we already synced it up - do nothing
		return
	}
	// and if valid, set it as new subjective head
	s.pending.Add(netHead)
	s.wantSync()
	log.Infow("new network head", "height", netHead.Height(), "hash", netHead.Hash())
}

// incomingNetworkHead processes new potential network headers.
// If the header valid, sets as new subjective header.
func (s *Syncer[H]) incomingNetworkHead(ctx context.Context, netHead H) pubsub.ValidationResult {
	// first of all, check the validity of the netHead
	res := s.validateHead(ctx, netHead)
	if res == pubsub.ValidationAccept {
		// and set it if valid
		s.setSubjectiveHead(ctx, netHead)
	}
	return res
}

// validateHead checks validity of the given header against the subjective head.
func (s *Syncer[H]) validateHead(ctx context.Context, new H) pubsub.ValidationResult {
	sbjHead, err := s.subjectiveHead(ctx)
	if err != nil {
		log.Errorw("getting subjective head during validation", "err", err)
		return pubsub.ValidationIgnore // local error, so ignore
	}
	// ignore header if it's from the past
	if new.Height() <= sbjHead.Height() {
		log.Warnw("received known network header",
			"current_height", sbjHead.Height(),
			"header_height", new.Height(),
			"header_hash", new.Hash())
		return pubsub.ValidationIgnore
	}
	// perform verification
	err = sbjHead.Verify(new)
	var verErr *header.VerifyError
	if errors.As(err, &verErr) {
		log.Errorw("invalid network header",
			"height_of_invalid", new.Height(),
			"hash_of_invalid", new.Hash(),
			"height_of_subjective", sbjHead.Height(),
			"hash_of_subjective", sbjHead.Hash(),
			"reason", verErr.Reason)
		return pubsub.ValidationReject
	}
	// and accept if the header is good
	return pubsub.ValidationAccept
}

// TODO(@Wondertan): We should request TrustingPeriod from the network's state params or
//  listen for network params changes to always have a topical value.

// isExpired checks if header is expired against trusting period.
func isExpired(header header.Header, period time.Duration) bool {
	expirationTime := header.Time().Add(period)
	return !expirationTime.After(time.Now())
}

// isRecent checks if header is recent against the given blockTime.
func isRecent(header header.Header, blockTime time.Duration) bool {
	return time.Since(header.Time()) <= blockTime+blockTime/2 // add half block time drift
}
