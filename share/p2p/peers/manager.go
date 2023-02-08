package peers

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

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

//TODO: address TODOs
//TODO: add debug logs
//TODO: simplify tests
// TODO: add GC for old / unwanted pools
//TODO: metrics
// - validation results rate[result_type, source:discovery/shrexsub]
// - peer retrival rate[shrexsub/discovery]
// - retrieval time hist
// - validation time hist
// - blacklisted peers count
// - blacklistedHash count

const (
	gcInterval = time.Second * 30

	ResultSuccess syncResult = iota
	ResultFail
	ResultPeerMisbehaved
)

var log = logging.Logger("shrex/peer_manager")

// Manager keeps track of peers coming from shrex.Sub and from discovery
type Manager struct {
	disc *discovery.Discovery
	// header subscription is necessary in order to validate the inbound eds hash
	headerSub libhead.Subscriber[*header.ExtendedHeader]
	shrexSub  shrexsub.PubSub
	host      host.Host

	poolsLock       sync.Mutex
	pools           map[string]*syncPool
	poolSyncTimeout time.Duration
	fullNodes       *pool

	// peers that misbehaved
	blacklistedPeers map[peer.ID]bool
	// hashes that are not in the chain
	blacklistedHashes map[string]bool

	cancel context.CancelFunc
	done   chan struct{}
}

type syncPool struct {
	*pool

	// isValidatedDataHash indicates if datahash was validated by receiving corresponding extended header from headerSub
	isValidatedDataHash atomic.Bool
	createdAt           time.Time
}

type (
	syncResult int
	DoneFunc   func(result syncResult)
)

func NewManager(
	headerSub libhead.Subscriber[*header.ExtendedHeader],
	host host.Host,
	shrexSub shrexsub.PubSub,
	discovery *discovery.Discovery,
	syncTimeout time.Duration,
) *Manager {
	s := &Manager{
		disc:              discovery,
		headerSub:         headerSub,
		shrexSub:          shrexSub,
		host:              host,
		pools:             make(map[string]*syncPool),
		poolSyncTimeout:   syncTimeout,
		fullNodes:         newPool(),
		blacklistedPeers:  make(map[peer.ID]bool),
		blacklistedHashes: make(map[string]bool),
		done:              make(chan struct{}),
	}

	discovery.WithOnPeersUpdate(
		func(peerID peer.ID, isAdded bool) {
			if isAdded {
				s.fullNodes.add(peerID)
				return
			}
			s.fullNodes.remove(peerID)
		})

	return s
}

func (s *Manager) Start() error {
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel

	err := s.shrexSub.Start(ctx)
	if err != nil {
		return fmt.Errorf("start shrexsub: %w", err)
	}

	err = s.shrexSub.AddValidator(s.validate)
	if err != nil {
		return fmt.Errorf("register validator: %w", err)
	}

	_, err = s.shrexSub.Subscribe()
	if err != nil {
		return fmt.Errorf("subscribe shrexsub: %w", err)
	}

	sub, err := s.headerSub.Subscribe()
	if err != nil {
		return fmt.Errorf("subscribe headersub: %w", err)
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

// GetPeer returns peer collected from shrex.Sub for given datahash if any available.
// If there is none, it will look for fullnodes collected from discovery. If there is no discovered
// full nodes, it will wait until any peer appear in either source or timeout happen.
// After fetching data using given peer, caller is required to call returned DoneFunc
func (s *Manager) GetPeer(
	ctx context.Context, datahash share.DataHash,
) (peer.ID, DoneFunc, error) {
	p := s.getOrCreatePool(datahash.String())
	p.markValidated()

	peerID, ok := p.tryGet()
	if ok {
		// some pools could still have blacklisted peers in storage
		if s.peerIsBlacklisted(peerID) {
			p.remove(peerID)
			return s.GetPeer(ctx, datahash)
		}
		return peerID, s.doneFunc(datahash, peerID), nil
	}

	// try to use full node address obtained from discovery
	peerID, ok = s.fullNodes.tryGet()
	if ok {
		return peerID, s.doneFunc(datahash, peerID), nil
	}

	// no peers are available right now, wait for the first one
	select {
	case peerID = <-p.getNext(ctx):
		return peerID, s.doneFunc(datahash, peerID), nil
	case peerID = <-s.fullNodes.getNext(ctx):
		return peerID, s.doneFunc(datahash, peerID), nil
	case <-ctx.Done():
		return "", nil, ctx.Err()
	}
}

func (s *Manager) doneFunc(datahash share.DataHash, peerID peer.ID) DoneFunc {
	return func(result syncResult) {
		switch result {
		case ResultSuccess:
			s.deletePool(datahash.String())
		case ResultFail:
		case ResultPeerMisbehaved:
			s.backlistPeer(peerID)
		}
	}
}

// subscribeHeader takes datahash from received header and validates corresponding peer pool
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
	// broadcasted messages should bypass the validation with Accept
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
	s.poolsLock.Lock()
	defer s.poolsLock.Unlock()

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

func (s *Manager) deletePool(datahash string) {
	s.poolsLock.Lock()
	defer s.poolsLock.Unlock()
	delete(s.pools, datahash)
}

func (s *Manager) backlistPeer(peerID peer.ID) {
	s.poolsLock.Lock()
	defer s.poolsLock.Unlock()
	s.blacklistedPeers[peerID] = true
	s.fullNodes.remove(peerID)
}

func (s *Manager) peerIsBlacklisted(peerID peer.ID) bool {
	s.poolsLock.Lock()
	defer s.poolsLock.Unlock()
	return s.blacklistedPeers[peerID]
}

func (s *Manager) hashIsBlacklisted(hash share.DataHash) bool {
	s.poolsLock.Lock()
	defer s.poolsLock.Unlock()
	return s.blacklistedHashes[hash.String()]
}

func (p *syncPool) markValidated() {
	p.isValidatedDataHash.Store(true)
}

func (s *Manager) GC(ctx context.Context) {
	ticker := time.NewTicker(gcInterval)
	for {
		s.cleanUP()

		select {
		case <-ticker.C:
		case <-ctx.Done():
			return
		}
	}
}

func (s *Manager) cleanUP() {
	s.poolsLock.Lock()
	defer s.poolsLock.Unlock()

	for h, p := range s.pools {
		if time.Now().Sub(p.createdAt) > s.poolSyncTimeout && !p.isValidatedDataHash.Load() {
			log.Debug("blacklisting datahash with all corresponding peers",
				"datahash", h,
				"peer_list", p.peersList)
			// blacklist hash
			delete(s.pools, h)
			s.blacklistedHashes[h] = true

			// blacklist peers
			for _, peer := range p.peersList {
				s.blacklistedPeers[peer] = true
				s.fullNodes.remove(peer)
			}
		}
	}
}
