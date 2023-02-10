package peers

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/libp2p/go-libp2p/p2p/net/conngater"

	logging "github.com/ipfs/go-log/v2"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/celestiaorg/celestia-node/header"
	libhead "github.com/celestiaorg/celestia-node/libs/header"
	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/availability/discovery"
	"github.com/celestiaorg/celestia-node/share/p2p/shrexsub"
)

const (
	ResultSuccess syncResult = iota
	ResultFail
	ResultPeerMisbehaved

	gcInterval = time.Second * 30
)

var log = logging.Logger("shrex/peer-manager")

// Manager keeps track of peers coming from shrex.Sub and from discovery
type Manager struct {
	lock sync.Mutex

	// header subscription is necessary in order to validate the inbound eds hash
	headerSub libhead.Subscriber[*header.ExtendedHeader]
	shrexSub  *shrexsub.PubSub
	disc      *discovery.Discovery
	host      host.Host
	connGater *conngater.BasicConnectionGater

	// pools collecting peers from shrexSub
	pools           map[string]*syncPool
	poolSyncTimeout time.Duration
	// fullNodes collects full nodes peer.ID found via discovery
	fullNodes *pool

	// hashes that are not in the chain
	blacklistedHashes map[string]bool

	cancel context.CancelFunc
	done   chan struct{}
}

// DoneFunc is a function performs an action based on the given syncResult.
type DoneFunc func(result syncResult)

type syncResult int

type syncPool struct {
	*pool

	// isValidatedDataHash indicates if datahash was validated by receiving corresponding extended
	// header from headerSub
	isValidatedDataHash atomic.Bool
	isSynced            atomic.Bool
	createdAt           time.Time
}

func NewManager(
	headerSub libhead.Subscriber[*header.ExtendedHeader],
	shrexSub *shrexsub.PubSub,
	discovery *discovery.Discovery,
	host host.Host,
	connGater *conngater.BasicConnectionGater,
	syncTimeout time.Duration,
) *Manager {
	s := &Manager{
		headerSub:         headerSub,
		shrexSub:          shrexSub,
		disc:              discovery,
		connGater:         connGater,
		host:              host,
		pools:             make(map[string]*syncPool),
		poolSyncTimeout:   syncTimeout,
		fullNodes:         newPool(),
		blacklistedHashes: make(map[string]bool),
		done:              make(chan struct{}),
	}

	discovery.WithOnPeersUpdate(
		func(peerID peer.ID, isAdded bool) {
			if isAdded && !s.peerIsBlacklisted(peerID) {
				s.fullNodes.add(peerID)
				return
			}
			s.fullNodes.remove(peerID)
		})

	return s
}

func (s *Manager) Start(startCtx context.Context) error {
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel

	err := s.shrexSub.Start(startCtx)
	if err != nil {
		return fmt.Errorf("starting shrexsub: %w", err)
	}

	err = s.shrexSub.AddValidator(s.validate)
	if err != nil {
		return fmt.Errorf("registering validator: %w", err)
	}

	_, err = s.shrexSub.Subscribe()
	if err != nil {
		return fmt.Errorf("subscribing to shrexsub: %w", err)
	}

	sub, err := s.headerSub.Subscribe()
	if err != nil {
		return fmt.Errorf("subscribing to headersub: %w", err)
	}

	go s.disc.EnsurePeers(ctx)
	go s.subscribeHeader(ctx, sub)
	go s.GC(ctx)

	return nil
}

func (s *Manager) Stop(ctx context.Context) error {
	s.cancel()
	select {
	case <-s.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Peer returns peer collected from shrex.Sub for given datahash if any available.
// If there is none, it will look for fullnodes collected from discovery. If there is no discovered
// full nodes, it will wait until any peer appear in either source or timeout happen.
// After fetching data using given peer, caller is required to call returned DoneFunc
func (s *Manager) Peer(
	ctx context.Context, datahash share.DataHash,
) (peer.ID, DoneFunc, error) {
	p := s.getOrCreatePool(datahash.String())
	p.markValidated()

	// first, check if a peer is available for the given datahash
	peerID, ok := p.tryGet()
	if ok {
		// some pools could still have blacklisted peers in storage
		if s.peerIsBlacklisted(peerID) {
			p.remove(peerID)
			return s.Peer(ctx, datahash)
		}
		return peerID, s.doneFunc(datahash, peerID), nil
	}

	// if no peer for datahash is currently available, try to use full node
	// obtained from discovery
	peerID, ok = s.fullNodes.tryGet()
	if ok {
		return peerID, s.doneFunc(datahash, peerID), nil
	}

	// no peers are available right now, wait for the first one
	select {
	case peerID = <-p.next(ctx):
		return peerID, s.doneFunc(datahash, peerID), nil
	case peerID = <-s.fullNodes.next(ctx):
		return peerID, s.doneFunc(datahash, peerID), nil
	case <-ctx.Done():
		return "", nil, ctx.Err()
	}
}

func (s *Manager) doneFunc(datahash share.DataHash, peerID peer.ID) DoneFunc {
	return func(result syncResult) {
		switch result {
		case ResultSuccess:
			s.getOrCreatePool(datahash.String()).markSynced()
		case ResultFail:
		case ResultPeerMisbehaved:
			s.blacklistPeers(peerID)
		}
	}
}

// subscribeHeader takes datahash from received header and validates corresponding peer pool.
func (s *Manager) subscribeHeader(ctx context.Context, headerSub libhead.Subscription[*header.ExtendedHeader]) {
	defer close(s.done)
	defer headerSub.Cancel()

	for {
		h, err := headerSub.NextHeader(ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return
			}
			log.Errorw("get next header from sub", "err", err)
			continue
		}

		s.getOrCreatePool(h.DataHash.String()).markValidated()
	}
}

// Validate will collect peer.ID into corresponding peer pool
func (s *Manager) validate(ctx context.Context, peerID peer.ID, hash share.DataHash) pubsub.ValidationResult {
	// messages broadcasted from self should bypass the validation with Accept
	if peerID == s.host.ID() {
		return pubsub.ValidationAccept
	}

	// punish peer for sending invalid hash if it has misbehaved in the past
	if s.hashIsBlacklisted(hash) || s.peerIsBlacklisted(peerID) {
		return pubsub.ValidationReject
	}

	s.getOrCreatePool(hash.String()).add(peerID)
	return pubsub.ValidationIgnore
}

func (s *Manager) getOrCreatePool(datahash string) *syncPool {
	s.lock.Lock()
	defer s.lock.Unlock()

	p, ok := s.pools[datahash]
	if !ok {
		p = &syncPool{
			pool:      newPool(),
			createdAt: time.Now(),
		}
		s.pools[datahash] = p
	}

	return p
}

func (s *Manager) blacklistPeers(peerIDs ...peer.ID) {
	for _, peerID := range peerIDs {
		s.fullNodes.remove(peerID)
		// add peer to the blacklist, so we can't connect to it in the future.
		err := s.connGater.BlockPeer(peerID)
		if err != nil {
			log.Debugw("blocking peer failed", "peer_id", peerID, "err", err)
		}
		// close connections to peer.
		err = s.host.Network().ClosePeer(peerID)
		if err != nil {
			log.Debugw("closing connection with peer failed", "peer_id", peerID, "err", err)
		}
	}
}

func (s *Manager) peerIsBlacklisted(peerID peer.ID) bool {
	return !s.connGater.InterceptPeerDial(peerID)
}

func (s *Manager) hashIsBlacklisted(hash share.DataHash) bool {
	s.lock.Lock()
	defer s.lock.Unlock()
	return s.blacklistedHashes[hash.String()]
}

func (s *Manager) GC(ctx context.Context) {
	ticker := time.NewTicker(gcInterval)
	defer ticker.Stop()

	var blacklist []peer.ID
	for {
		blacklist = s.cleanUp()
		s.blacklistPeers(blacklist...)

		select {
		case <-ticker.C:
		case <-ctx.Done():
			return
		}
	}
}

func (s *Manager) cleanUp() []peer.ID {
	s.lock.Lock()
	defer s.lock.Unlock()

	addToBlackList := make(map[peer.ID]struct{})
	for h, p := range s.pools {
		if time.Since(p.createdAt) > s.poolSyncTimeout && !p.isValidatedDataHash.Load() {
			log.Debug("blacklisting datahash with all corresponding peers",
				"datahash", h,
				"peer_list", p.peersList)
			// blacklist hash
			delete(s.pools, h)
			s.blacklistedHashes[h] = true

			// blacklist peers
			for _, peer := range p.peersList {
				addToBlackList[peer] = struct{}{}
			}
		}
	}

	blacklist := make([]peer.ID, 0, len(addToBlackList))
	for peerID := range addToBlackList {
		blacklist = append(blacklist, peerID)
	}
	return blacklist
}

func (p *syncPool) markSynced() {
	old := (*unsafe.Pointer)(unsafe.Pointer(&p.pool))
	// release pointer to old pool to be garbage collected
	atomic.StorePointer(old, unsafe.Pointer(newPool()))
	p.isSynced.Store(true)
}

func (p *syncPool) markValidated() {
	p.isValidatedDataHash.Store(true)
}

func (p *syncPool) add(peers ...peer.ID) {
	if !p.isSynced.Load() {
		p.pool.add(peers...)
	}
}
