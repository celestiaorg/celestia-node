package peers

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	logging "github.com/ipfs/go-log/v2"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/celestiaorg/celestia-node/header"
	libhead "github.com/celestiaorg/celestia-node/libs/header"
	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/availability/discovery"
	"github.com/celestiaorg/celestia-node/share/p2p/shrexsub"
)

var (
	log = logging.Logger("shrex/peers")
)

//go:generate mockgen -destination=mocks/subscription.go -package=mocks . HeaderSub
type HeaderSub = libhead.Subscription[*header.ExtendedHeader]

// Manager keeps track of peers coming from shrex.Sub and discovery
type Manager struct {
	disc      discovery.Discovery
	headerSub HeaderSub

	m               *sync.Mutex
	pools           map[hashStr]syncPool
	poolSyncTimeout time.Duration
	fullNodes       *pool

	cancel context.CancelFunc
	done   chan struct{}
}

type hashStr string

// syncPool accumulates peers from shrex.Sub validators and controls message retransmission.
// It will unlock the validator only if there is a proof that header with given datahash exist in a
// chain
type syncPool struct {
	pool *pool

	isValidDataHash    *atomic.Bool
	validatorWaitCh    chan struct{}
	validatorWaitTimer *time.Timer
}

func NewManager(
	shrexSub *shrexsub.PubSub,
	headerSub HeaderSub,
	discovery discovery.Discovery,
	syncTimeout time.Duration,
) (*Manager, error) {
	s := &Manager{
		disc:            discovery,
		headerSub:       headerSub,
		m:               new(sync.Mutex),
		pools:           make(map[hashStr]syncPool),
		poolSyncTimeout: syncTimeout,
		fullNodes:       newPool(),
		done:            make(chan struct{}),
	}

	discovery.WithOnPeersUpdate(
		func(peerID peer.ID, isAdded bool) {
			if isAdded {
				s.fullNodes.add(peerID)
				return
			}
			s.fullNodes.remove(peerID)
		})

	err := shrexSub.AddValidator(s.validate)
	return s, err
}

func (s *Manager) Start() {
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	go s.disc.EnsurePeers(ctx)
	go s.subscribeHeader(ctx)
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

// GetPeer attempts to get a peer obtained from shrex.Sub for given datahash if any.
// If there is none, it will try fullnodes collected from discovery. And if there is still none, it
// will wait until any peer appear in either source or timeout happen.
func (s *Manager) GetPeer(ctx context.Context, datahash string) (peer.ID, error) {
	p := s.getOrCreateValidatedPool(hashStr(datahash))
	peerID, ok := p.pool.tryGet()
	if ok {
		return peerID, nil
	}

	// try fullnodes obtained from discovery
	peerID, ok = s.fullNodes.tryGet()
	if ok {
		return peerID, nil
	}

	// no peers available, wait for the first one
	select {
	case peerID = <-p.pool.waitNext(ctx):
		return peerID, nil
	case peerID = <-s.fullNodes.waitNext(ctx):
		return peerID, nil
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

func (s *Manager) RemovePeers(datahash string, ids ...peer.ID) {
	p := s.getOrCreateValidatedPool(hashStr(datahash))
	p.pool.remove(ids...)
}

// subscribeHeader subscribes to headerSub and
func (s *Manager) subscribeHeader(ctx context.Context) {
	defer close(s.done)
	defer s.headerSub.Cancel()

	for {
		h, err := s.headerSub.NextHeader(ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return
			}
			log.Errorw("get next header from sub", "err", err)
			continue
		}

		s.getOrCreateValidatedPool(hashStr(h.DataHash.String()))
	}
}

func (s *Manager) getOrCreateValidatedPool(key hashStr) syncPool {
	s.m.Lock()
	defer s.m.Unlock()

	p, ok := s.pools[key]
	if !ok {
		// save as already validated
		p.pool = newPool()
		p.isValidDataHash = new(atomic.Bool)
		p.isValidDataHash.Store(true)
		s.pools[key] = p
		return p
	}

	// check if there are validators waiting
	if p.isValidDataHash.CompareAndSwap(false, true) {
		// datahash is valid, so unlock all awaiting Validators.
		// if unable to stop the timer, the channel was already closed by afterfunc
		if p.validatorWaitTimer.Stop() {
			close(p.validatorWaitCh)
		}
	}
	return p
}

// Validator will block until header with given datahash received. This behavior opens an attack
// vector of multiple fake datahash spam, that will grow amount of hanging routines in node. To
// address this, validator should be reworked to be non-blocking, with retransmission being invoked
// in sync manner from another routine upon header discovery.
func (s *Manager) validate(ctx context.Context, peerID peer.ID, hash share.DataHash) pubsub.ValidationResult {
	p := s.getOrCreateUnvalidatedPool(hash)
	p.pool.add(peerID)

	if p.isValidDataHash.Load() {
		// datahash is valid, allow the pubsub to retransmit the message by returning Accept
		return pubsub.ValidationAccept
	}

	// wait for header to sync within timeout.
	select {
	case <-p.validatorWaitCh:
		if p.isValidDataHash.Load() {
			// headerSub found corresponding ExtendedHeader for dataHash,
			// retransmit the message by returning Accept
			return pubsub.ValidationAccept
		}

		// no corresponding header was received for given datahash in time,
		// highly unlikely block with given datahash exist in chain, reject msg and punish the peer
		s.deletePool(hashStr(hash.String()))
		return pubsub.ValidationReject
	case <-ctx.Done():
		return pubsub.ValidationIgnore
	}
}

func (s *Manager) getOrCreateUnvalidatedPool(dataHash share.DataHash) syncPool {
	s.m.Lock()
	defer s.m.Unlock()

	key := hashStr(dataHash.String())
	t, ok := s.pools[key]
	if ok {
		return t
	}

	// create pool in non-validated state
	waitCh := make(chan struct{})
	t = syncPool{
		pool:            newPool(),
		isValidDataHash: new(atomic.Bool),
		validatorWaitCh: waitCh,
		validatorWaitTimer: time.AfterFunc(s.poolSyncTimeout, func() {
			close(waitCh)
		}),
	}
	s.pools[key] = t
	return t
}

func (s *Manager) deletePool(key hashStr) {
	s.m.Lock()
	defer s.m.Unlock()
	delete(s.pools, key)
}
