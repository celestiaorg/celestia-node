package peers

import (
	"context"
	"errors"
	"fmt"
	"github.com/celestiaorg/celestia-node/share/p2p/shrexsub"
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
)

// TODO: metrics
// - current number of locked validators
// - validation results rate[result_type, source:discovery/shrexsub]
// - peer retrival rate[source]
// - retrieval time hist
// - validation time hist
// - discovery fallback amount

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
	broadcast shrexsub.BroadcastFn
	ownPeerID peer.ID

	poolsLock       sync.Mutex
	pools           map[string]*syncPool
	poolSyncTimeout time.Duration
	fullNodes       *pool

	cancel context.CancelFunc
	done   chan struct{}
}

// syncPool accumulates peers from shrex.Sub validators and controls message retransmission.
// It will unlock the validator if two conditions are met:
//  1. an ExtendedHeader that corresponds to the data hash was received and verified by the node
//  2. the EDS corresponding to the data hash was synced by the node
type syncPool struct {
	*pool

	isValidDataHash    atomic.Bool
	validationDeadline time.Time
	blacklisted           bool
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

type DoneFunc func(ctx context.Context, result syncResult) error

type syncResult int

// GetPeer returns peer collected from shrex.Sub for given datahash if any available.
// If there is none, it will look for fullnodes collected from discovery. If there is no discovered
// full nodes, it will wait until any peer appear in either source or timeout happen.
// After fetching data using given peer, caller is required to call returned DoneFunc
func (s *Manager) GetPeer(
	ctx context.Context, datahash share.DataHash,
) (peer.ID, DoneFunc, error) {
	p := s.getOrCreatePool(datahash.String())
	p.markValidated()
	fmt.Println("Pool get ", len(p.pool.peersList))

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
		fmt.Println("nothing came", len(p.pool.peersList))
		return "", nil, ctx.Err()
	}
}

func (s *Manager) doneFunc(datahash share.DataHash, peerID peer.ID) DoneFunc {
	return func(ctx context.Context, result syncResult) error {
		switch result {
		case ResultSuccess:
			fmt.Println("BROADCAST", peerID.String())
			s.deletePool(datahash.String())
			err := s.broadcast(ctx, datahash)
			if err != nil {
				return fmt.Errorf("broadcast new message; %w", err)
			}
		case ResultFail:
		case ResultPeerMisbehaved:
			fmt.Println("MISBEHAVED")
			// TODO: signal to peers Validator to return Reject
			s.RemovePeers(datahash, peerID)
			s.fullNodes.remove(peerID)
		}
		return nil
	}
}

// RemovePeers removes peers for given datahash from store
func (s *Manager) RemovePeers(datahash share.DataHash, ids ...peer.ID) {
	s.getOrCreatePool(datahash.String()).remove(ids...)
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

		s.getOrCreatePool(h.DataHash.String()).markValidated()
	}
}

// Validate will block until header with given datahash received. This behavior opens an attack
// vector of multiple fake datahash spam, that will grow amount of hanging routines in node. To
// address this, validator should be reworked to be non-blocking, with retransmission being invoked
// in sync manner from another routine upon header discovery.
func (s *Manager) Validate(ctx context.Context, peerID peer.ID, hash share.DataHash) pubsub.ValidationResult {
	fmt.Println("VALIDATE", hash.String(), peerID.String())
	if peerID == s.ownPeerID {
		return pubsub.ValidationAccept
	}

	p := s.getOrCreatePool(hash.String())
	//TODO: check blacklisted
	p.add(peerID)
	return pubsub.ValidationIgnore
}

// TODO: find better name
func (s *Manager) getOrCreatePool(datahash string) *syncPool {
	s.poolsLock.Lock()
	defer s.poolsLock.Unlock()

	p, ok := s.pools[datahash]
	if !ok {
		// create pool in non-validated state
		p = &syncPool{
			pool: newPool(),
			validationDeadline: time.Now().Add(s.poolSyncTimeout),
		}
		s.pools[datahash] = p
		fmt.Println("create")
	}

	return p
}

func (s *Manager) deletePool(datahash string) {
	s.poolsLock.Lock()
	defer s.poolsLock.Unlock()
	delete(s.pools, datahash)
}

func (p *syncPool) markValidated() {
	p.isValidDataHash.Store(true)
}

func (s *Manager)