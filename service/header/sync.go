package header

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/libp2p/go-libp2p-core/peer"
	pubsub "github.com/libp2p/go-libp2p-pubsub"

	tmbytes "github.com/tendermint/tendermint/libs/bytes"
)

// TODO:
//  1. Sync protocol for peers to exchange their local heads on connect
//  2. If we are far from peers but within an unbonding period - trigger sync automatically
//  3. If we are beyond the unbonding period - request Local Head + 1 header from trusted and hardcoded peer
//  automatically and continue sync until know head.
//  4. Limit amount of requests on server side
//  5. Sync status and till sync is done.
//  6. Cache even unverifiable headers from the future, but don't resend them with ignore.
//  7. Retry requesting headers
//  8. Hardcode genesisHash

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

	// pending keeps ranges of valid headers rcvd from the network awaiting to be appended to store
	pending *ranges

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
		triggerSync: make(chan struct{}),
		pending:     newRanges(),
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
	s.mustSync()
	return nil
}

func (s *Syncer) wantSync() {
	select {
	case s.triggerSync <- struct{}{}:
	default:
	}
}

func (s *Syncer) mustSync() {
	select {
	case s.triggerSync <- struct{}{}:
	case <-s.ctx.Done():
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
	for {
		if ctx.Err() != nil {
			return
		}

		sbjHead, err := s.subjectiveHead(ctx)
		if err != nil {
			log.Errorw("getting subjective head", "err", err)
			return
		}

		objHead, err := s.objectiveHead(ctx, sbjHead)
		if err != nil {
			log.Errorw("getting objective head", "err", err)
			return
		}

		if sbjHead.Height >= objHead.Height {
			// we are now synced
			log.Info("synced headers")
			return
		}

		err = s.syncDiff(ctx, sbjHead, objHead)
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
		// we might be missing some headers, so save verified header to pending cache
		s.pending.Add(maybeHead)
		// and trigger sync to catch-up
		s.wantSync()
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

// subjectiveHead tries to get head locally and if not exists requests by trusted hash.
func (s *Syncer) subjectiveHead(ctx context.Context) (*ExtendedHeader, error) {
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

// objectiveHead gets the objective network head.
func (s *Syncer) objectiveHead(ctx context.Context, sbj *ExtendedHeader) (*ExtendedHeader, error) {
	phead := s.pending.Head()
	if phead != nil && phead.Height >= sbj.Height {
		return phead, nil
	}

	return s.exchange.RequestHead(ctx)
}

// TODO(@Wondertan): Number of headers that can be requested at once. Either make this configurable or,
//  find a proper rationale for constant.
var requestSize uint64 = 256

// syncDiff requests headers from knownHead up to new head.
func (s *Syncer) syncDiff(ctx context.Context, knownHead, newHead *ExtendedHeader) error {
	start, end := uint64(knownHead.Height+1), uint64(newHead.Height)
	for start < end {
		amount := end - start
		if amount > requestSize {
			amount = requestSize
		}

		headers, err := s.getHeaders(ctx, start, amount)
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

// getHeaders gets headers from either remote peers or from local cached of headers rcvd by PubSub
func (s *Syncer) getHeaders(ctx context.Context, start, amount uint64) ([]*ExtendedHeader, error) {
	// short-circuit if nothing in pending cache to avoid unnecessary allocation below
	if _, ok := s.pending.NextWithin(start, start+amount); !ok {
		return s.exchange.RequestHeaders(ctx, start, amount)
	}

	end, out := start+amount, make([]*ExtendedHeader, 0, amount)
	for start < end {
		// if we have some range cached - use it
		if r, ok := s.pending.NextWithin(start, end); ok {
			// first, request everything between start and found range
			hs, err := s.exchange.RequestHeaders(ctx, start, r.Start-start)
			if err != nil {
				return nil, err
			}
			out = append(out, hs...)
			start += uint64(len(hs))

			// than, apply cached range
			cached := r.Before(end)
			out = append(out, cached...)
			start += uint64(len(cached))

			// repeat, as there might be multiple caches
			continue
		}

		// fetch the leftovers
		hs, err := s.exchange.RequestHeaders(ctx, start, start-end)
		if err != nil {
			return nil, err
		}

		return append(out, hs...), nil
	}

	return out, nil
}

type ranges struct {
	head   *ExtendedHeader
	ranges []*_range
	lk     sync.Mutex // no need for RWMutex as there is only one reader
}

func newRanges() *ranges {
	return new(ranges)
}

func (rs *ranges) Head() *ExtendedHeader {
	rs.lk.Lock()
	defer rs.lk.Unlock()
	return rs.head
}

func (rs *ranges) Add(h *ExtendedHeader) {
	rs.lk.Lock()
	defer rs.lk.Unlock()

	// short-circuit if header is from the past
	if rs.head.Height >= h.Height {
		// TODO(@Wondertan): Technically, we can still apply the header:
		//  * Headers here are verified, so we can trust them
		//  * PubSub does not guarantee the ordering of msgs
		//    * So there might be a case where ordering is broken
		//    * Even considering the delay(block time) with which new headers are generated
		//    * But rarely
		//  Would be still nice to implement
		log.Warnf("rcvd headers in wrong order")
		return
	}

	// if the new header is adjacent to head
	if h.Height == rs.head.Height+1 {
		// append it to the last known range
		rs.ranges[len(rs.ranges)-1].Append(h)
	} else {
		// otherwise, start a new range
		rs.ranges = append(rs.ranges, newRange(h))

		// it is possible to miss a header or few from PubSub, due to quick disconnects or sleep
		// once we start rcving them again we save those in new range
		// so 'Syncer.getHeaders' can fetch what was missed
	}

	// we know the new header is higher than head, so it as a new head
	rs.head = h
}

func (rs *ranges) NextWithin(start, end uint64) (*_range, bool) {
	r, ok := rs.Next()
	if !ok {
		return nil, false
	}

	if r.Start >= start && r.Start < end {
		return r, true
	}

	return nil, false
}

func (rs *ranges) Next() (*_range, bool) {
	rs.lk.Lock()
	defer rs.lk.Unlock()

	for {
		if len(rs.ranges) == 0 {
			return nil, false
		}

		out := rs.ranges[0]
		if !out.Empty() {
			return out, true
		}

		rs.ranges = rs.ranges[1:]
	}
}

type _range struct {
	Start uint64

	headers []*ExtendedHeader
}

func newRange(h *ExtendedHeader) *_range {
	return &_range{
		Start:   uint64(h.Height),
		headers: []*ExtendedHeader{h},
	}
}

func (r *_range) Append(h ...*ExtendedHeader) {
	r.headers = append(r.headers, h...)
}

func (r *_range) Empty() bool {
	return len(r.headers) == 0
}

func (r *_range) Before(end uint64) []*ExtendedHeader {
	amnt := uint64(len(r.headers))
	if r.Start+amnt > end {
		amnt = end - r.Start
	}

	out := r.headers[:amnt-1]
	r.headers = r.headers[amnt:]
	return out
}
