package peers

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/celestiaorg/celestia-node/header"

	logging "github.com/ipfs/go-log/v2"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/availability/discovery"
	"github.com/celestiaorg/celestia-node/share/p2p/shrexsub"
)

var (
	log = logging.Logger("shrex/peers")
)

// Manager keeps track of peers coming from shrex.Sub and discovery
type Manager struct {
	disc      discovery.Discovery
	headerSub header.Subscription

	m               sync.Mutex
	pools           map[hashStr]syncPool
	poolSyncTimeout time.Duration
	fullNodes       *pool

	cancel context.CancelFunc
	done   chan struct{}
}

type hashStr = string

func NewManager(
	shrexSub *shrexsub.PubSub,
	headerSub header.Subscription,
	discovery discovery.Discovery,
	syncTimeout time.Duration,
) (*Manager, error) {
	s := &Manager{
		disc:            discovery,
		headerSub:       headerSub,
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

type DoneFunc func(success bool)

// GetPeer attempts to get a peer obtained from shrex.Sub for given datahash if any.
// If there is none, it will try fullnodes collected from discovery. And if there is still none, it
// will wait until any peer appear in either source or timeout happen.
// After fetching data using given peer, caller is required to call returned DoneFunc
func (s *Manager) GetPeer(
	ctx context.Context, datahash share.DataHash,
) (peer.ID, DoneFunc, error) {
	p := s.getOrCreateValidatedPool(datahash.String())
	peerID, ok := p.pool.tryGet()
	if ok {
		return peerID, s.doneFunc(datahash, peerID), nil
	}

	// try fullnodes obtained from discovery
	peerID, ok = s.fullNodes.tryGet()
	if ok {
		return peerID, s.doneFunc(datahash, peerID), nil
	}

	// no peers available, wait for the first one
	select {
	case peerID = <-p.pool.waitNext(ctx):
		return peerID, s.doneFunc(datahash, peerID), nil
	case peerID = <-s.fullNodes.waitNext(ctx):
		return peerID, s.doneFunc(datahash, peerID), nil
	case <-ctx.Done():
		return "", s.doneFunc(datahash, peerID), ctx.Err()
	}
}

func (s *Manager) doneFunc(datahash share.DataHash, peerID peer.ID) DoneFunc {
	return func(success bool) {
		if success {
			s.sampled(datahash)
			return
		}
		s.RemovePeers(datahash, peerID)
	}
}

// sampled marks datahash as sampled if not yet marked, to release waiting validator and retransmit
// the message via shrex.Sub
func (s *Manager) sampled(datahash share.DataHash) {
	p := s.getOrCreateValidatedPool(datahash.String())
	if p.isSampled.CompareAndSwap(false, true) {
		close(p.waitSamplingCh)
	}
}

// RemovePeers removes peers for given datahash from store
func (s *Manager) RemovePeers(datahash share.DataHash, ids ...peer.ID) {
	p := s.getOrCreateValidatedPool(datahash.String())
	p.pool.remove(ids...)
}

// subscribeHeader marks pool as validated when its datahash corresponds to headers received from
// headerSub.
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

		s.getOrCreateValidatedPool(h.DataHash.String())
	}
}

func (s *Manager) getOrCreateValidatedPool(key hashStr) syncPool {
	s.m.Lock()
	defer s.m.Unlock()

	p, ok := s.pools[key]
	if !ok {
		// save as already validated
		p = newSyncPool()
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
	p := s.getOrCreateUnvalidatedPool(hash.String())
	p.pool.add(peerID)

	// only first validator call is responsible for retransmit, all subsequent call should return ignore
	if !p.firstPeer.CompareAndSwap(false, true) {
		return pubsub.ValidationIgnore
	}

	if !p.isValidDataHash.Load() {
		if valid := p.waitValidation(ctx); !valid {
			// no corresponding header was received for given datahash in time,
			// highly unlikely block with given datahash exist in chain, reject msg and punish the peer
			s.deletePool(hash.String())
			return pubsub.ValidationReject
		}
	}

	if sampled := p.waitSampling(ctx); sampled {
		// headerSub found corresponding ExtendedHeader for dataHash,
		// wait for it to be sampled, before retransmission
		return pubsub.ValidationAccept
	}
	return pubsub.ValidationIgnore
}

func (s *Manager) getOrCreateUnvalidatedPool(key hashStr) syncPool {
	s.m.Lock()
	defer s.m.Unlock()

	p, ok := s.pools[key]
	if !ok {
		// create pool in non-validated state
		p = newSyncPool()
		p.validatorWaitCh = make(chan struct{})
		p.validatorWaitTimer = time.AfterFunc(s.poolSyncTimeout, func() {
			close(p.validatorWaitCh)
		})

		s.pools[key] = p
	}

	return p
}

func (s *Manager) deletePool(key hashStr) {
	s.m.Lock()
	defer s.m.Unlock()
	delete(s.pools, key)
}
