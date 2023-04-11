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
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/net/conngater"

	libhead "github.com/celestiaorg/go-header"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/availability/discovery"
	"github.com/celestiaorg/celestia-node/share/p2p/shrexsub"
)

const (
	// ResultSuccess indicates operation was successful and no extra action is required
	ResultSuccess result = iota
	// ResultSynced will save the status of pool as "synced" and will remove peers from it
	ResultSynced
	// ResultCooldownPeer will put returned peer on cooldown, meaning it won't be available by Peer
	// method for some time
	ResultCooldownPeer
	// ResultBlacklistPeer will blacklist peer. Blacklisted peers will be disconnected and blocked from
	// any p2p communication in future by libp2p Gater
	ResultBlacklistPeer
)

var log = logging.Logger("shrex/peer-manager")

// Manager keeps track of peers coming from shrex.Sub and from discovery
type Manager struct {
	lock   sync.Mutex
	params Parameters

	// header subscription is necessary in order to validate the inbound eds hash
	headerSub libhead.Subscriber[*header.ExtendedHeader]
	shrexSub  *shrexsub.PubSub
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

	cancel context.CancelFunc
	done   chan struct{}
}

// DoneFunc updates internal state depending on call results. Should be called once per returned
// peer from Peer method
type DoneFunc func(result)

type result int

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
		params:            params,
		headerSub:         headerSub,
		shrexSub:          shrexSub,
		connGater:         connGater,
		host:              host,
		pools:             make(map[string]*syncPool),
		blacklistedHashes: make(map[string]bool),
		done:              make(chan struct{}),
	}

	s.fullNodes = newPool(s.params.PeerCooldown)

	discovery.WithOnPeersUpdate(
		func(peerID peer.ID, isAdded bool) {
			if isAdded {
				if s.isBlacklistedPeer(peerID) {
					log.Debugw("got blacklisted peer from discovery", "peer", peerID)
					return
				}
				log.Debugw("added to full nodes", "peer", peerID)
				s.fullNodes.add(peerID)
				return
			}

			log.Debugw("removing peer from discovered full nodes", "peer", peerID)
			s.fullNodes.remove(peerID)
		})

	return s, nil
}

func (m *Manager) Start(startCtx context.Context) error {
	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel

	err := m.shrexSub.AddValidator(m.validate)
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

	go m.subscribeHeader(ctx, headerSub)
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
		// some pools could still have blacklisted peers in storage
		if m.isBlacklistedPeer(peerID) {
			log.Debugw("removing blacklisted peer from pool", "hash", datahash.String(),
				"peer", peerID.String())
			p.remove(peerID)
			return m.Peer(ctx, datahash)
		}
		log.Debugw("returning shrex-sub peer", "hash", datahash.String(),
			"peer", peerID.String())
		return peerID, m.doneFunc(datahash, peerID, false), nil
	}

	// if no peer for datahash is currently available, try to use full node
	// obtained from discovery
	peerID, ok = m.fullNodes.tryGet()
	if ok {
		log.Debugw("got peer from full nodes discovery pool", "peer", peerID, "datahash", datahash.String())
		return peerID, m.doneFunc(datahash, peerID, true), nil
	}

	// no peers are available right now, wait for the first one
	select {
	case peerID = <-p.next(ctx):
		log.Debugw("got peer from shrexSub pool after wait", "peer", peerID, "datahash", datahash.String())
		return peerID, m.doneFunc(datahash, peerID, false), nil
	case peerID = <-m.fullNodes.next(ctx):
		log.Debugw("got peer from discovery pool after wait", "peer", peerID, "datahash", datahash.String())
		return peerID, m.doneFunc(datahash, peerID, true), nil
	case <-ctx.Done():
		return "", nil, ctx.Err()
	}
}

func (m *Manager) doneFunc(datahash share.DataHash, peerID peer.ID, fromFull bool) DoneFunc {
	return func(result result) {
		log.Debugw("set peer status",
			"peer", peerID,
			"datahash", datahash.String(),
			"result", result)
		switch result {
		case ResultSuccess:
		case ResultSynced:
			m.getOrCreatePool(datahash.String()).markSynced()
		case ResultCooldownPeer:
			m.getOrCreatePool(datahash.String()).putOnCooldown(peerID)
			if fromFull {
				m.fullNodes.putOnCooldown(peerID)
			}
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
		m.validatedPool(h.DataHash.String())

		// store first header for validation purposes
		if m.initialHeight.CompareAndSwap(0, uint64(h.Height())) {
			log.Debugw("stored initial height", "height", h.Height())
		}
	}
}

// validate will collect peer.ID into corresponding peer pool
func (m *Manager) validate(_ context.Context, peerID peer.ID, msg shrexsub.Notification) pubsub.ValidationResult {

	// messages broadcast from self should bypass the validation with Accept
	if peerID == m.host.ID() {
		log.Debugw("received datahash from self", "datahash", msg.DataHash.String())
		return pubsub.ValidationAccept
	}

	// punish peer for sending invalid hash if it has misbehaved in the past
	if m.isBlacklistedHash(msg.DataHash) {
		log.Debugw("received blacklisted hash, reject validation", "peer", peerID, "datahash", msg.DataHash.String())
		return pubsub.ValidationReject
	}

	if m.isBlacklistedPeer(peerID) {
		log.Debugw("received message from blacklisted peer, reject validation",
			"peer", peerID,
			"datahash", msg.DataHash.String())
		return pubsub.ValidationReject
	}

	if msg.Height == 0 {
		log.Debugw("received message with 0 height", "peer", peerID)
		return pubsub.ValidationReject
	}

	if msg.Height < m.initialHeight.Load() {
		// we can use peers from discovery for headers before the first one from headerSub
		// if we allow pool creation for those headers, there is chance the pool will not be validated in
		// time and will be false-positively trigger blacklisting of hash and all peers that sent msgs for
		// that hash
		log.Debugw("received message for past header", "peer", peerID, "datahash", msg.DataHash.String())
		return pubsub.ValidationIgnore
	}

	p := m.getOrCreatePool(msg.DataHash.String())
	p.headerHeight.Store(msg.Height)
	p.add(peerID)
	log.Debugw("got hash from shrex-sub", "peer", peerID, "datahash", msg.DataHash.String())
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

func (m *Manager) blacklistPeers(peerIDs ...peer.ID) {
	log.Debugw("blacklisting peers", "peers", peerIDs)

	if !m.params.EnableBlackListing {
		return
	}
	for _, peerID := range peerIDs {
		m.fullNodes.remove(peerID)
		// add peer to the blacklist, so we can't connect to it in the future.
		err := m.connGater.BlockPeer(peerID)
		if err != nil {
			log.Warnw("failed tp block peer", "peer", peerID, "err", err)
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
	}
	return p
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
			m.blacklistPeers(blacklist...)
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
				"datahash", h,
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

func (p *syncPool) markSynced() {
	p.isSynced.Store(true)
	old := (*unsafe.Pointer)(unsafe.Pointer(&p.pool))
	// release pointer to old pool to free up memory
	atomic.StorePointer(old, unsafe.Pointer(newPool(time.Second)))
}

func (p *syncPool) add(peers ...peer.ID) {
	if !p.isSynced.Load() {
		p.pool.add(peers...)
	}
}
