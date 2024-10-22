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
	"github.com/libp2p/go-libp2p/core/event"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/host/eventbus"
	"github.com/libp2p/go-libp2p/p2p/net/conngater"

	libhead "github.com/celestiaorg/go-header"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/shwap/p2p/shrex/shrexsub"
)

const (
	// ResultNoop indicates operation was successful and no extra action is required
	ResultNoop result = "result_noop"
	// ResultCooldownPeer will put returned peer on cooldown, meaning it won't be available by Peer
	// method for some time
	ResultCooldownPeer = "result_cooldown_peer"
	// ResultBlacklistPeer will blacklist peer. Blacklisted peers will be disconnected and blocked from
	// any p2p communication in future by libp2p Gater
	ResultBlacklistPeer = "result_blacklist_peer"

	// eventbusBufSize is the size of the buffered channel to handle
	// events in libp2p
	eventbusBufSize = 32

	// storedPoolsAmount is the amount of pools for recent headers that will be stored in the peer
	// manager
	storedPoolsAmount = 10
)

type result string

var log = logging.Logger("shrex/peer-manager")

// Manager keeps track of peers coming from shrex.Sub and from discovery
type Manager struct {
	lock   sync.Mutex
	params Parameters

	// identifies the type of peers the manager
	// is managing
	tag string

	// header subscription is necessary in order to Validate the inbound eds hash
	headerSub libhead.Subscriber[*header.ExtendedHeader]
	shrexSub  *shrexsub.PubSub
	host      host.Host
	connGater *conngater.BasicConnectionGater

	// pools collecting peers from shrexSub and stores them by datahash
	pools map[string]*syncPool

	// initialHeight is the height of the first header received from headersub
	initialHeight atomic.Uint64
	// messages from shrex.Sub with height below storeFrom will be ignored, since we don't need to
	// track peers for those headers
	storeFrom atomic.Uint64

	// nodes collects nodes' peer.IDs found via discovery
	nodes *pool

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
	// height is the height of the header that corresponds to datahash
	height uint64
	// createdAt is the syncPool creation time
	createdAt time.Time
}

func NewManager(
	params Parameters,
	host host.Host,
	connGater *conngater.BasicConnectionGater,
	tag string,
	options ...Option,
) (*Manager, error) {
	if err := params.Validate(); err != nil {
		return nil, err
	}

	s := &Manager{
		params:                params,
		connGater:             connGater,
		host:                  host,
		pools:                 make(map[string]*syncPool),
		blacklistedHashes:     make(map[string]bool),
		headerSubDone:         make(chan struct{}),
		disconnectedPeersDone: make(chan struct{}),
		tag:                   tag,
	}

	for _, opt := range options {
		err := opt(s)
		if err != nil {
			return nil, err
		}
	}

	s.nodes = newPool(s.params.PeerCooldown)
	return s, nil
}

func (m *Manager) Start(startCtx context.Context) error {
	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel

	// pools will only be populated with senders of shrexsub notifications if the WithShrexSubPools
	// option is used.
	if m.shrexSub == nil && m.headerSub == nil {
		return nil
	}

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

	go m.subscribeHeader(ctx, headerSub)
	go m.subscribeDisconnectedPeers(ctx, sub)
	go m.GC(ctx)
	return nil
}

func (m *Manager) Stop(ctx context.Context) error {
	m.cancel()

	if err := m.metrics.close(); err != nil {
		log.Warnw("closing metrics", "err", err)
	}

	// we do not need to wait for headersub and disconnected peers to finish
	// here, since they were never started
	if m.headerSub == nil && m.shrexSub == nil {
		return nil
	}

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
// If there is none, it will look for nodes collected from discovery. If there is no discovered
// nodes, it will wait until any peer appear in either source or timeout happen.
// After fetching data using given peer, caller is required to call returned DoneFunc using
// appropriate result value
func (m *Manager) Peer(ctx context.Context, datahash share.DataHash, height uint64,
) (peer.ID, DoneFunc, error) {
	p := m.validatedPool(datahash.String(), height)

	// first, check if a peer is available for the given datahash
	peerID, ok := p.tryGet()
	if ok {
		if m.removeIfUnreachable(p, peerID) {
			return m.Peer(ctx, datahash, height)
		}
		return m.newPeer(ctx, datahash, peerID, sourceShrexSub, p.len(), 0)
	}

	// if no peer for datahash is currently available, try to use node
	// obtained from discovery
	peerID, ok = m.nodes.tryGet()
	if ok {
		return m.newPeer(ctx, datahash, peerID, sourceDiscoveredNodes, m.nodes.len(), 0)
	}

	// no peers are available right now, wait for the first one
	start := time.Now()
	select {
	case peerID = <-p.next(ctx):
		if m.removeIfUnreachable(p, peerID) {
			return m.Peer(ctx, datahash, height)
		}
		return m.newPeer(ctx, datahash, peerID, sourceShrexSub, p.len(), time.Since(start))
	case peerID = <-m.nodes.next(ctx):
		return m.newPeer(ctx, datahash, peerID, sourceDiscoveredNodes, m.nodes.len(), time.Since(start))
	case <-ctx.Done():
		return "", nil, ctx.Err()
	}
}

// UpdateNodePool is called by discovery when new node is discovered or removed.
func (m *Manager) UpdateNodePool(peerID peer.ID, isAdded bool) {
	if isAdded {
		if m.isBlacklistedPeer(peerID) {
			log.Debugw("got blacklisted peer from discovery", "peer", peerID.String())
			return
		}
		m.nodes.add(peerID)
		log.Debugw("added to discovered nodes pool", "peer", peerID)
		return
	}

	log.Debugw("removing peer from discovered nodes pool", "peer", peerID.String())
	m.nodes.remove(peerID)
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
		case ResultCooldownPeer:
			if source == sourceDiscoveredNodes {
				m.nodes.putOnCooldown(peerID)
				return
			}
			m.getPool(datahash.String()).putOnCooldown(peerID)
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
		m.validatedPool(h.DataHash.String(), h.Height())

		// store first header for validation purposes
		if m.initialHeight.CompareAndSwap(0, h.Height()) {
			log.Debugw("stored initial height", "height", h.Height())
		}

		// update storeFrom if header height
		m.storeFrom.Store(uint64(max(0, int(h.Height())-storedPoolsAmount)))
		log.Debugw("updated lowest stored height", "height", h.Height())
	}
}

// subscribeDisconnectedPeers subscribes to libp2p connectivity events and removes disconnected
// peers from nodes pool
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
			// listen to disconnect event to remove peer from nodes pool
			connStatus := e.(event.EvtPeerConnectednessChanged)
			if connStatus.Connectedness == network.NotConnected {
				peer := connStatus.Peer
				if m.nodes.has(peer) {
					log.Debugw("peer disconnected, removing from discovered nodes pool",
						"peer", peer.String())
					m.nodes.remove(peer)
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

	if msg.Height < m.storeFrom.Load() {
		logger.Debug("received message for past header")
		return pubsub.ValidationIgnore
	}

	p := m.getOrCreatePool(msg.DataHash.String(), msg.Height)
	logger.Debugw("got hash from shrex-sub")

	p.add(peerID)
	if p.isValidatedDataHash.Load() {
		// add peer to discovered nodes pool only if datahash has been already validated
		m.nodes.add(peerID)
	}
	return pubsub.ValidationIgnore
}

func (m *Manager) getPool(datahash string) *syncPool {
	m.lock.Lock()
	defer m.lock.Unlock()
	return m.pools[datahash]
}

func (m *Manager) getOrCreatePool(datahash string, height uint64) *syncPool {
	m.lock.Lock()
	defer m.lock.Unlock()

	p, ok := m.pools[datahash]
	if !ok {
		p = &syncPool{
			height:    height,
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

		m.nodes.remove(peerID)
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

func (m *Manager) validatedPool(hashStr string, height uint64) *syncPool {
	p := m.getOrCreatePool(hashStr, height)
	if p.isValidatedDataHash.CompareAndSwap(false, true) {
		log.Debugw("pool marked validated", "datahash", hashStr)
		// if pool is proven to be valid, add all collected peers to discovered nodes
		m.nodes.add(p.peers()...)
	}
	return p
}

// removeIfUnreachable removes peer from some pool if it is blacklisted or disconnected
func (m *Manager) removeIfUnreachable(pool *syncPool, peerID peer.ID) bool {
	if m.isBlacklistedPeer(peerID) || !m.nodes.has(peerID) {
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
		if p.isValidatedDataHash.Load() {
			// remove pools that are outdated
			if p.height < m.storeFrom.Load() {
				delete(m.pools, h)
			}
			continue
		}

		// can't validate datahashes below initial height
		if p.height < m.initialHeight.Load() {
			delete(m.pools, h)
			continue
		}

		// find pools that are not validated in time
		if time.Since(p.createdAt) > m.params.PoolValidationTimeout {
			delete(m.pools, h)

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
