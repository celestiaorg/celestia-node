package peers

import (
	"context"
	"errors"
	"sync"
	"time"

	logging "github.com/ipfs/go-log/v2"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/celestiaorg/celestia-node/header"
	libhead "github.com/celestiaorg/celestia-node/libs/header"
	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/availability/discovery"
)

const (
	ResultSuccess syncResult = iota
	ResultFail
	ResultPeerMisbehaved
)

var log = logging.Logger("shrex/peers")

// Manager keeps track of peers coming from shrex.Sub and from discovery
type Manager struct {
	disc *discovery.Discovery
	// header subscription is necessary in order to validate the inbound eds hash
	headerSub libhead.Subscriber[*header.ExtendedHeader]

	poolsLock       sync.Mutex
	pools           map[string]*syncPool
	poolSyncTimeout time.Duration
	fullNodes       *pool

	cancel context.CancelFunc
	done   chan struct{}
}

func NewManager(
	headerSub libhead.Subscriber[*header.ExtendedHeader],
	discovery *discovery.Discovery,
	syncTimeout time.Duration,
) *Manager {
	s := &Manager{
		disc:            discovery,
		headerSub:       headerSub,
		pools:           make(map[string]*syncPool),
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

	return s
}

func (s *Manager) Start() error {
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel

	sub, err := s.headerSub.Subscribe()
	if err != nil {
		return err
	}
	go s.disc.EnsurePeers(ctx)
	go s.subscribeHeader(ctx, sub)

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

type DoneFunc func(result syncResult)

type syncResult int

// GetPeer returns peer collected from shrex.Sub for given datahash if any available.
// If there is none, it will look for fullnodes collected from discovery. If there is no discovered
// full nodes, it will wait until any peer appear in either source or timeout happen.
// After fetching data using given peer, caller is required to call returned DoneFunc
func (s *Manager) GetPeer(
	ctx context.Context, datahash share.DataHash,
) (peer.ID, DoneFunc, error) {
	p := s.getOrCreateValidatedPool(datahash.String())
	peerID, ok := p.tryGet()
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
			s.markSynced(datahash)
		case ResultFail:
		case ResultPeerMisbehaved:
			// TODO: signal to peers Validator to return Reject
			s.RemovePeers(datahash, peerID)
			s.fullNodes.remove(peerID)
		}
	}
}

// markSynced marks datahash as synced if not yet marked, to release waiting validator and
// retransmit the message via shrex.Sub
func (s *Manager) markSynced(datahash share.DataHash) {
	p := s.getOrCreateValidatedPool(datahash.String())
	if p.isSynced.CompareAndSwap(false, true) {
		close(p.waitSyncCh)
	}
}

// RemovePeers removes peers for given datahash from store
func (s *Manager) RemovePeers(datahash share.DataHash, ids ...peer.ID) {
	p := s.getOrCreateValidatedPool(datahash.String())
	p.remove(ids...)
}

// subscribeHeader marks pool as validated when its datahash corresponds to a header received from
// headerSub.
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

		s.getOrCreateValidatedPool(h.DataHash.String())
	}
}

func (s *Manager) getOrCreateValidatedPool(datahash string) *syncPool {
	s.poolsLock.Lock()
	defer s.poolsLock.Unlock()

	p, ok := s.pools[datahash]
	if !ok {
		p = newSyncPool()

		// save as already validated
		p.isValidDataHash.Store(true)
		s.pools[datahash] = p
		return p
	}

	// if not yet validated, there is validator waiting that needs to be released
	p.indicateValid()
	return p
}

// Validate will block until header with given datahash received. This behavior opens an attack
// vector of multiple fake datahash spam, that will grow amount of hanging routines in node. To
// address this, validator should be reworked to be non-blocking, with retransmission being invoked
// in sync manner from another routine upon header discovery.
func (s *Manager) Validate(ctx context.Context, peerID peer.ID, hash share.DataHash) pubsub.ValidationResult {
	p := s.getOrCreateUnvalidatedPool(hash.String())
	p.add(peerID)

	// check if validation is required
	if !p.isValidDataHash.Load() {
		if valid := p.waitValidation(ctx); !valid {
			// no corresponding header was received for given datahash in time,
			// highly unlikely block with given datahash exist in chain, reject msg and punish the peer
			s.deletePool(hash.String())
			return pubsub.ValidationReject
		}
	}

	// headerSub found corresponding ExtendedHeader for dataHash,
	// wait for it to be synced, before retransmission
	if synced := p.WaitSync(ctx); synced {
		// block with given datahash was synced, allow the pubsub to retransmit the message by
		// returning Accept
		return pubsub.ValidationAccept
	}
	return pubsub.ValidationIgnore
}

func (s *Manager) getOrCreateUnvalidatedPool(datahash string) *syncPool {
	s.poolsLock.Lock()
	defer s.poolsLock.Unlock()

	p, ok := s.pools[datahash]
	if !ok {
		// create pool in non-validated state
		p = newSyncPool()
		p.validatorWaitCh = make(chan struct{})
		p.validatorWaitTimer = time.AfterFunc(s.poolSyncTimeout, func() {
			close(p.validatorWaitCh)
		})

		s.pools[datahash] = p
	}
	return p
}

func (s *Manager) deletePool(datahash string) {
	s.poolsLock.Lock()
	defer s.poolsLock.Unlock()
	delete(s.pools, datahash)
}
