package peers

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	logging "github.com/ipfs/go-log/v2"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/event"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/host/eventbus"
	"github.com/libp2p/go-libp2p/p2p/net/conngater"

	libhead "github.com/celestiaorg/go-header"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/p2p/discovery"
	"github.com/celestiaorg/celestia-node/share/p2p/shrexsub"
)

const (
	// ResultNoop indicates operation was successful and no extra action is required
	ResultNoop result = "result_noop"
	// ResultSynced will save the status of pool as "synced" and will remove peers from it
	ResultSynced = "result_synced"
	// ResultCooldownPeer will put returned peer on cooldown, meaning it won't be available by Peer
	// method for some time
	ResultCooldownPeer = "result_cooldown_peer"
	// ResultBlacklistPeer will blacklist peer. Blacklisted peers will be disconnected and blocked from
	// any p2p communication in future by libp2p Gater
	ResultBlacklistPeer = "result_blacklist_peer"

	// eventbusBufSize is the size of the buffered channel to handle
	// events in libp2p
	eventbusBufSize = 32
)

type result string

var log = logging.Logger("shrex/peer-manager")

// Manager keeps track of peers coming from shrex.Sub and from discovery
type Manager struct {
	lock   sync.Mutex
	params Parameters

	// header subscription is necessary in order to Validate the inbound eds hash
	headerSub libhead.Subscriber[*header.ExtendedHeader]
	shrexSub  *shrexsub.PubSub
	disc      *discovery.Discovery
	host      host.Host
	connGater *conngater.BasicConnectionGater

	// pools collecting peers from shrexSub
	pools map[string]*syncPool
	// messages from shrex.Sub with height below initialHeight will be ignored, since we don't need to
	// track peers for those headers
	initialHeight atomic.Uint64

	// fullNodes collects full nodes peer.ID found via discovery
	fullNodes *pool

	// hashes that are not in the chain
	blacklistedHashes map[string]bool

	metrics *metrics

	headerSubDone         chan struct{}
	disconnectedPeersDone chan struct{}
	cancel                context.CancelFunc
}

// DoneFunc updates internal state depending on call results. Should be called once per returned
// peer from Peer method
type DoneFunc func(result)

type syncPool struct {
	*pool

	// isValidatedDataHash indicates if datahash was validated by receiving corresponding extended
	// header from headerSub
	isValidatedDataHash atomic.Bool
	// headerHeight is the height of header corresponding to syncpool
	headerHeight atomic.Uint64
	// isSynced will be true if DoneFunc was called with ResultSynced. It indicates that given datahash
	// was synced and peer-manager no longer need to keep peers for it
	isSynced atomic.Bool
	// createdAt is the syncPool creation time
	createdAt time.Time
}

func NewManager(
	params Parameters,
	headerSub libhead.Subscriber[*header.ExtendedHeader],
	shrexSub *shrexsub.PubSub,
	discovery *discovery.Discovery,
	host host.Host,
	connGater *conngater.BasicConnectionGater,
) (*Manager, error) {
	if err := params.Validate(); err != nil {
		return nil, err
	}

	s := &Manager{
		params:                params,
		headerSub:             headerSub,
		shrexSub:              shrexSub,
		connGater:             connGater,
		disc:                  discovery,
		host:                  host,
		pools:                 make(map[string]*syncPool),
		blacklistedHashes:     make(map[string]bool),
		headerSubDone:         make(chan struct{}),
		disconnectedPeersDone: make(chan struct{}),
	}

	s.fullNodes = newPool(s.params.PeerCooldown)

	discovery.WithOnPeersUpdate(
		func(peerID peer.ID, isAdded bool) {
			if isAdded {
				if s.isBlacklistedPeer(peerID) {
					log.Debugw("got blacklisted peer from discovery", "peer", peerID.String())
					return
				}
				s.fullNodes.add(peerID)
				log.Debugw("added to full nodes", "peer", peerID)
				return
			}

			log.Debugw("removing peer from discovered full nodes", "peer", peerID.String())
			s.fullNodes.remove(peerID)
		})

	return s, nil
}

func (m *Manager) Start(startCtx context.Context) error {
	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel

	validatorFn := m.metrics.validationObserver(m.Validate)
	err := m.shrexSub.AddValidator(validatorFn)
	if err != nil {
		return fmt.Errorf("registering validator: %w", err)
	}

	err = m.shrexSub.Start(startCtx)
	if err != nil {
		return fmt.Errorf("starting shrexsub: %w", err)
	}

	headerSub, err := m.headerSub.Subscribe()
	if err != nil {
		return fmt.Errorf("subscribing to headersub: %w", err)
	}

	sub, err := m.host.EventBus().Subscribe(&event.EvtPeerConnectednessChanged{}, eventbus.BufSize(eventbusBufSize))
	if err != nil {
		return fmt.Errorf("subscribing to libp2p events: %w", err)
	}

	go m.subscribeDisconnectedPeers(ctx, sub)
	go m.subscribeHeader(ctx, headerSub)
	go m.GC(ctx)

	return nil
}

func (m *Manager) Stop(ctx context.Context) error {
	m.cancel()

	select {
	case <-m.headerSubDone:
	case <-ctx.Done():
		return ctx.Err()
	}

	select {
	case <-m.disconnectedPeersDone:
	case <-ctx.Done():
		return ctx.Err()
	}

	return nil
}

// Peer returns peer collected from shrex.Sub for given datahash if any available.
// If there is none, it will look for full nodes collected from discovery. If there is no discovered
// full nodes, it will wait until any peer appear in either source or timeout happen.
// After fetching data using given peer, caller is required to call returned DoneFunc using
// appropriate result value
func (m *Manager) Peer(
	ctx context.Context, datahash share.DataHash,
) (peer.ID, DoneFunc, error) {
	p := m.validatedPool(datahash.String())

	// first, check if a peer is available for the given datahash
	peerID, ok := p.tryGet()
	if ok {
		if m.removeIfUnreachable(p, peerID) {
			return m.Peer(ctx, datahash)
		}
		return m.newPeer(ctx, datahash, peerID, sourceShrexSub, p.len(), 0)
	}

	// if no peer for datahash is currently available, try to use full node
	// obtained from discovery
	peerID, ok = m.fullNodes.tryGet()
	if ok {
		return m.newPeer(ctx, datahash, peerID, sourceFullNodes, m.fullNodes.len(), 0)
	}

	// no peers are available right now, wait for the first one
	start := time.Now()
	select {
	case peerID = <-p.next(ctx):
		if m.removeIfUnreachable(p, peerID) {
			return m.Peer(ctx, datahash)
		}
		return m.newPeer(ctx, datahash, peerID, sourceShrexSub, p.len(), time.Since(start))
	case peerID = <-m.fullNodes.next(ctx):
		return m.newPeer(ctx, datahash, peerID, sourceFullNodes, m.fullNodes.len(), time.Since(start))
	case <-ctx.Done():
		return "", nil, ctx.Err()
	}
}

func (m *Manager) newPeer(
	ctx context.Context,
	datahash share.DataHash,
	peerID peer.ID,
	source peerSource,
	poolSize int,
	waitTime time.Duration,
) (peer.ID, DoneFunc, error) {
	log.Debugw("got peer",
		"hash", datahash.String(),
		"peer", peerID.String(),
		"source", source,
		"pool_size", poolSize,
		"wait (s)", waitTime)
	m.metrics.observeGetPeer(ctx, source, poolSize, waitTime)
	return peerID, m.doneFunc(datahash, peerID, source), nil
}

func (m *Manager) doneFunc(datahash share.DataHash, peerID peer.ID, source peerSource) DoneFunc {
	return func(result result) {
		log.Debugw("set peer result",
			"hash", datahash.String(),
			"peer", peerID.String(),
			"source", source,
			"result", result)
		m.metrics.observeDoneResult(source, result)
		switch result {
		case ResultNoop:
		case ResultSynced:
			m.markPoolAsSynced(datahash.String())
		case ResultCooldownPeer:
			if source == sourceFullNodes {
				m.fullNodes.putOnCooldown(peerID)
				return
			}
			m.getOrCreatePool(datahash.String()).putOnCooldown(peerID)
		case ResultBlacklistPeer:
			m.blacklistPeers(reasonMisbehave, peerID)
		}
	}
}

// subscribeHeader takes datahash from received header and validates corresponding peer pool.
func (m *Manager) subscribeHeader(ctx context.Context, headerSub libhead.Subscription[*header.ExtendedHeader]) {
	defer close(m.headerSubDone)
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
		m.validatedPool(h.DataHash.String())

		// store first header for validation purposes
		if m.initialHeight.CompareAndSwap(0, h.Height()) {
			log.Debugw("stored initial height", "height", h.Height())
		}
	}
}

// subscribeDisconnectedPeers subscribes to libp2p connectivity events and removes disconnected
// peers from full nodes pool
func (m *Manager) subscribeDisconnectedPeers(ctx context.Context, sub event.Subscription) {
	defer close(m.disconnectedPeersDone)
	defer sub.Close()
	for {
		select {
		case <-ctx.Done():
			return
		case e, ok := <-sub.Out():
			if !ok {
				log.Fatal("Subscription for connectedness events is closed.") //nolint:gocritic
				return
			}
			// listen to disconnect event to remove peer from full nodes pool
			connStatus := e.(event.EvtPeerConnectednessChanged)
			if connStatus.Connectedness == network.NotConnected {
				peer := connStatus.Peer
				if m.fullNodes.has(peer) {
					log.Debugw("peer disconnected, removing from full nodes", "peer", peer.String())
					m.fullNodes.remove(peer)
				}
			}
		}
	}
}

// Validate will collect peer.ID into corresponding peer pool
func (m *Manager) Validate(_ context.Context, peerID peer.ID, msg shrexsub.Notification) pubsub.ValidationResult {
	logger := log.With("peer", peerID.String(), "hash", msg.DataHash.String())

	// messages broadcast from self should bypass the validation with Accept
	if peerID == m.host.ID() {
		logger.Debug("received datahash from self")
		return pubsub.ValidationAccept
	}

	// punish peer for sending invalid hash if it has misbehaved in the past
	if m.isBlacklistedHash(msg.DataHash) {
		logger.Debug("received blacklisted hash, reject validation")
		return pubsub.ValidationReject
	}

	if m.isBlacklistedPeer(peerID) {
		logger.Debug("received message from blacklisted peer, reject validation")
		return pubsub.ValidationReject
	}

	if msg.Height == 0 {
		logger.Debug("received message with 0 height")
		return pubsub.ValidationReject
	}

	if msg.Height < m.initialHeight.Load() {
		// we can use peers from discovery for headers before the first one from headerSub
		// if we allow pool creation for those headers, there is chance the pool will not be validated in
		// time and will be false-positively trigger blacklisting of hash and all peers that sent msgs for
		// that hash
		logger.Debug("received message for past header")
		return pubsub.ValidationIgnore
	}

	p := m.getOrCreatePool(msg.DataHash.String())
	p.headerHeight.Store(msg.Height)
	logger.Debugw("got hash from shrex-sub")

	p.add(peerID)
	if p.isValidatedDataHash.Load() {
		// add peer to full nodes pool only if datahash has been already validated
		m.fullNodes.add(peerID)
	}
	return pubsub.ValidationIgnore
}

func (m *Manager) getOrCreatePool(datahash string) *syncPool {
	m.lock.Lock()
	defer m.lock.Unlock()

	p, ok := m.pools[datahash]
	if !ok {
		p = &syncPool{
			pool:      newPool(m.params.PeerCooldown),
			createdAt: time.Now(),
		}
		m.pools[datahash] = p
	}

	return p
}

func (m *Manager) blacklistPeers(reason blacklistPeerReason, peerIDs ...peer.ID) {
	m.metrics.observeBlacklistPeers(reason, len(peerIDs))

	for _, peerID := range peerIDs {
		// blacklisted peers will be logged regardless of EnableBlackListing whether option being is
		// enabled, until blacklisting is not properly tested and enabled by default.
		log.Debugw("blacklisting peer", "peer", peerID.String(), "reason", reason)
		if !m.params.EnableBlackListing {
			continue
		}

		m.fullNodes.remove(peerID)
		// add peer to the blacklist, so we can't connect to it in the future.
		err := m.connGater.BlockPeer(peerID)
		if err != nil {
			log.Warnw("failed to block peer", "peer", peerID, "err", err)
		}
		// close connections to peer.
		err = m.host.Network().ClosePeer(peerID)
		if err != nil {
			log.Warnw("failed to close connection with peer", "peer", peerID, "err", err)
		}
	}
}

func (m *Manager) isBlacklistedPeer(peerID peer.ID) bool {
	return !m.connGater.InterceptPeerDial(peerID)
}

func (m *Manager) isBlacklistedHash(hash share.DataHash) bool {
	m.lock.Lock()
	defer m.lock.Unlock()
	return m.blacklistedHashes[hash.String()]
}

func (m *Manager) validatedPool(hashStr string) *syncPool {
	p := m.getOrCreatePool(hashStr)
	if p.isValidatedDataHash.CompareAndSwap(false, true) {
		log.Debugw("pool marked validated", "datahash", hashStr)
		// if pool is proven to be valid, add all collected peers to full nodes
		m.fullNodes.add(p.peers()...)
	}
	return p
}

// removeIfUnreachable removes peer from some pool if it is blacklisted or disconnected
func (m *Manager) removeIfUnreachable(pool *syncPool, peerID peer.ID) bool {
	if m.isBlacklistedPeer(peerID) || !m.fullNodes.has(peerID) {
		log.Debugw("removing outdated peer from pool", "peer", peerID.String())
		pool.remove(peerID)
		return true
	}
	return false
}

func (m *Manager) GC(ctx context.Context) {
	ticker := time.NewTicker(m.params.GcInterval)
	defer ticker.Stop()

	var blacklist []peer.ID
	for {
		select {
		case <-ticker.C:
		case <-ctx.Done():
			return
		}

		blacklist = m.cleanUp()
		if len(blacklist) > 0 {
			m.blacklistPeers(reasonInvalidHash, blacklist...)
		}
	}
}

func (m *Manager) cleanUp() []peer.ID {
	if m.initialHeight.Load() == 0 {
		// can't blacklist peers until initialHeight is set
		return nil
	}

	m.lock.Lock()
	defer m.lock.Unlock()

	addToBlackList := make(map[peer.ID]struct{})
	for h, p := range m.pools {
		if !p.isValidatedDataHash.Load() && time.Since(p.createdAt) > m.params.PoolValidationTimeout {
			delete(m.pools, h)
			if p.headerHeight.Load() < m.initialHeight.Load() {
				// outdated pools could still be valid even if not validated, no need to blacklist
				continue
			}
			log.Debug("blacklisting datahash with all corresponding peers",
				"hash", h,
				"peer_list", p.peersList)
			// blacklist hash
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

func (m *Manager) markPoolAsSynced(datahash string) {
	p := m.getOrCreatePool(datahash)
	if p.isSynced.CompareAndSwap(false, true) {
		p.isSynced.Store(true)
		old := (*unsafe.Pointer)(unsafe.Pointer(&p.pool))
		// release pointer to old pool to free up memory
		atomic.StorePointer(old, unsafe.Pointer(newPool(time.Second)))
	}
}

func (p *syncPool) add(peers ...peer.ID) {
	if !p.isSynced.Load() {
		p.pool.add(peers...)
	}
}
