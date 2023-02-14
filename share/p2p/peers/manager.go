package peers

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

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
	// ResultSuccess will save the status of pool as "synced" and will remove peers from it
	ResultSuccess syncResult = iota
	// ResultCooldownPeer will put returned peer on cooldown, meaning it won't be available by Peer
	// method for some time
	ResultCooldownPeer
	// ResultBlacklistPeer will blacklist peer. Blacklisted peers will be disconnected and blocked from
	// any p2p communication in future by libp2p Gater
	ResultBlacklistPeer

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
	pools            map[string]*syncPool
	poolSyncTimeout  time.Duration
	peerCooldownTime time.Duration
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
	createdAt           time.Time
}

func NewManager(
	headerSub libhead.Subscriber[*header.ExtendedHeader],
	shrexSub *shrexsub.PubSub,
	discovery *discovery.Discovery,
	host host.Host,
	connGater *conngater.BasicConnectionGater,
	syncTimeout time.Duration,
	peerCooldownTime time.Duration,
) *Manager {
	s := &Manager{
		headerSub:         headerSub,
		shrexSub:          shrexSub,
		disc:              discovery,
		connGater:         connGater,
		host:              host,
		pools:             make(map[string]*syncPool),
		poolSyncTimeout:   syncTimeout,
		fullNodes:         newPool(peerCooldownTime),
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

func (m *Manager) Start(startCtx context.Context) error {
	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel

	err := m.shrexSub.Start(startCtx)
	if err != nil {
		return fmt.Errorf("starting shrexsub: %w", err)
	}

	err = m.shrexSub.AddValidator(m.validate)
	if err != nil {
		return fmt.Errorf("registering validator: %w", err)
	}

	_, err = m.shrexSub.Subscribe()
	if err != nil {
		return fmt.Errorf("subscribing to shrexsub: %w", err)
	}

	sub, err := m.headerSub.Subscribe()
	if err != nil {
		return fmt.Errorf("subscribing to headersub: %w", err)
	}

	go m.disc.EnsurePeers(ctx)
	go m.subscribeHeader(ctx, sub)
	go m.GC(ctx)

	return nil
}

func (m *Manager) Stop(ctx context.Context) error {
	m.cancel()
	select {
	case <-m.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Peer returns peer collected from shrex.Sub for given datahash if any available.
// If there is none, it will look for fullnodes collected from discovery. If there is no discovered
// full nodes, it will wait until any peer appear in either source or timeout happen.
// After fetching data using given peer, caller is required to call returned DoneFunc
func (m *Manager) Peer(
	ctx context.Context, datahash share.DataHash,
) (peer.ID, DoneFunc, error) {
	p := m.getOrCreatePool(datahash.String())
	p.markValidated()

	// first, check if a peer is available for the given datahash
	peerID, ok := p.tryGet()
	if ok {
		// some pools could still have blacklisted peers in storage
		if m.peerIsBlacklisted(peerID) {
			p.remove(peerID)
			return m.Peer(ctx, datahash)
		}
		return peerID, m.doneFunc(datahash, peerID), nil
	}

	// if no peer for datahash is currently available, try to use full node
	// obtained from discovery
	peerID, ok = m.fullNodes.tryGet()
	if ok {
		return peerID, m.doneFunc(datahash, peerID), nil
	}

	// no peers are available right now, wait for the first one
	select {
	case peerID = <-p.next(ctx):
		return peerID, m.doneFunc(datahash, peerID), nil
	case peerID = <-m.fullNodes.next(ctx):
		return peerID, m.doneFunc(datahash, peerID), nil
	case <-ctx.Done():
		return "", nil, ctx.Err()
	}
}

func (m *Manager) doneFunc(datahash share.DataHash, peerID peer.ID) DoneFunc {
	return func(result syncResult) {
		switch result {
		case ResultSuccess:
			m.deletePool(datahash.String())
		case ResultCooldownPeer:
			m.getOrCreatePool(datahash.String()).putOnCooldown(peerID)
		case ResultBlacklistPeer:
			m.blacklistPeers(peerID)
		}
	}
}

// subscribeHeader takes datahash from received header and validates corresponding peer pool.
func (m *Manager) subscribeHeader(ctx context.Context, headerSub libhead.Subscription[*header.ExtendedHeader]) {
	defer close(m.done)
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

		m.getOrCreatePool(h.DataHash.String()).markValidated()
	}
}

// Validate will collect peer.ID into corresponding peer pool
func (m *Manager) validate(ctx context.Context, peerID peer.ID, hash share.DataHash) pubsub.ValidationResult {
	// messages broadcasted from self should bypass the validation with Accept
	if peerID == m.host.ID() {
		return pubsub.ValidationAccept
	}

	// punish peer for sending invalid hash if it has misbehaved in the past
	if m.hashIsBlacklisted(hash) || m.peerIsBlacklisted(peerID) {
		return pubsub.ValidationReject
	}

	m.getOrCreatePool(hash.String()).add(peerID)
	return pubsub.ValidationIgnore
}

func (m *Manager) getOrCreatePool(datahash string) *syncPool {
	m.lock.Lock()
	defer m.lock.Unlock()

	p, ok := m.pools[datahash]
	if !ok {
		p = &syncPool{
			pool:      newPool(m.peerCooldownTime),
			createdAt: time.Now(),
		}
		m.pools[datahash] = p
	}

	return p
}

func (m *Manager) deletePool(datahash string) {
	m.lock.Lock()
	defer m.lock.Unlock()
	delete(m.pools, datahash)
}

func (m *Manager) blacklistPeers(peerIDs ...peer.ID) {
	for _, peerID := range peerIDs {
		m.fullNodes.remove(peerID)
		// add peer to the blacklist, so we can't connect to it in the future.
		err := m.connGater.BlockPeer(peerID)
		if err != nil {
			log.Debugw("blocking peer failed", "peer_id", peerID, "err", err)
		}
		// close connections to peer.
		err = m.host.Network().ClosePeer(peerID)
		if err != nil {
			log.Debugw("closing connection with peer failed", "peer_id", peerID, "err", err)
		}
	}
}

func (m *Manager) peerIsBlacklisted(peerID peer.ID) bool {
	return !m.connGater.InterceptPeerDial(peerID)
}

func (m *Manager) hashIsBlacklisted(hash share.DataHash) bool {
	m.lock.Lock()
	defer m.lock.Unlock()
	return m.blacklistedHashes[hash.String()]
}

func (p *syncPool) markValidated() {
	p.isValidatedDataHash.Store(true)
}

func (m *Manager) GC(ctx context.Context) {
	ticker := time.NewTicker(gcInterval)
	defer ticker.Stop()

	var blacklist []peer.ID
	for {
		blacklist = m.cleanUp()
		m.blacklistPeers(blacklist...)

		select {
		case <-ticker.C:
		case <-ctx.Done():
			return
		}
	}
}

func (m *Manager) cleanUp() []peer.ID {
	m.lock.Lock()
	defer m.lock.Unlock()

	addToBlackList := make(map[peer.ID]struct{})
	for h, p := range m.pools {
		if time.Since(p.createdAt) > m.poolSyncTimeout && !p.isValidatedDataHash.Load() {
			log.Debug("blacklisting datahash with all corresponding peers",
				"datahash", h,
				"peer_list", p.peersList)
			// blacklist hash
			delete(m.pools, h)
			m.blacklistedHashes[h] = true

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
