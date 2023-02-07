package peers

import (
	"context"
	"fmt"
	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/availability/discovery"
	"github.com/celestiaorg/celestia-node/share/p2p/shrexsub"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/host"
	routingdisc "github.com/libp2p/go-libp2p/p2p/discovery/routing"
	mocknet "github.com/libp2p/go-libp2p/p2p/net/mock"
	"testing"
	"time"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/header"
	libhead "github.com/celestiaorg/celestia-node/libs/header"
)

// TODO: add broadcast to tests
func TestManager(t *testing.T) {
	t.Run("validated datahash, header comes first", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
		t.Cleanup(cancel)

		// create headerSub mock
		h := testHeader()
		headerSub := newSubLock(h, nil)

		// start test manager
		manager := testManager(ctx, headerSub)

		// wait until header is requested from header sub
		require.NoError(t, headerSub.wait(ctx, 1))

		doneCh := make(chan struct{})
		peerID := peer.ID("peer")
		// validator should return accept, since header is synced
		go func() {
			defer close(doneCh)
			validation := manager.Validate(ctx, peerID, h.DataHash.Bytes())
			require.Equal(t, pubsub.ValidationIgnore, validation)
		}()

		p, done, err := manager.GetPeer(ctx, h.DataHash.Bytes())
		done(ctx, ResultSuccess)
		require.NoError(t, err)
		require.Equal(t, peerID, p)

		// wait for validators to finish
		select {
		case <-doneCh:
		case <-ctx.Done():
			require.NoError(t, ctx.Err())
		}

		stopManager(t, manager)
	})

	t.Run("validated datahash, peers comes first", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*50)
		t.Cleanup(cancel)

		// create headerSub mock
		h := testHeader()
		headerSub := newSubLock(h, nil)

		// start test manager
		manager := testManager(ctx, headerSub)

		peers := []peer.ID{"peer1", "peer2", "peer3"}

		// spawn validator routine for every peer
		validationCh := make(chan pubsub.ValidationResult)
		for _, p := range peers {
			go func(peerID peer.ID) {
				select {
				case validationCh <- manager.Validate(ctx, peerID, h.DataHash.Bytes()):
				case <-ctx.Done():
					return
				}
			}(p)
		}

		// wait for peers to be collected in pool
		p := manager.getOrCreatePool(h.DataHash.String())
		waitPoolHasItems(ctx, t, p, 3)

		// release headerSub with validating header
		require.NoError(t, headerSub.wait(ctx, 1))
		// release sync lock by calling done function
		_, done, err := manager.GetPeer(ctx, h.DataHash.Bytes())
		done(ctx, ResultSuccess)
		require.NoError(t, err)

		// wait for validators to finish
		for i := 0; i < len(peers); i++ {
			select {
			case result := <-validationCh:
				require.Equal(t, pubsub.ValidationIgnore, result)
			case <-ctx.Done():
				require.NoError(t, ctx.Err())
			}
		}
		stopManager(t, manager)
	})

	t.Run("no datahash in timeout, reject validators", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
		t.Cleanup(cancel)

		// create headerSub mock
		h := testHeader()
		headerSub := newSubLock(h)

		// start test manager
		manager := testManager(ctx, headerSub)

		peers := []peer.ID{"peer1", "peer2", "peer3"}

		// set syncTimeout to 0 for fast rejects
		manager.poolSyncTimeout = 0

		// spawn validator routine for every peer
		validationCh := make(chan pubsub.ValidationResult)
		for _, p := range peers {
			go func(peerID peer.ID) {
				select {
				case validationCh <- manager.Validate(ctx, peerID, h.DataHash.Bytes()):
				case <-ctx.Done():
					return
				}
			}(p)
		}

		// wait for validators to finish
		for i := 0; i < len(peers); i++ {
			select {
			case result := <-validationCh:
				require.Equal(t, pubsub.ValidationIgnore, result)
			case <-ctx.Done():
				require.NoError(t, ctx.Err())
			}
		}

		stopManager(t, manager)
	})

	t.Run("no peers from shrex.Sub, get from discovery", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
		t.Cleanup(cancel)

		// create headerSub mock
		h := testHeader()
		headerSub := newSubLock(h)

		// start test manager
		manager := testManager(ctx, headerSub)

		// add peers to fullnodes, imitating discovery add
		peers := []peer.ID{"peer1", "peer2", "peer3"}
		manager.fullNodes.add(peers...)

		peerID, done, err := manager.GetPeer(ctx, h.DataHash.Bytes())
		done(ctx, ResultSuccess)
		require.NoError(t, err)
		require.Contains(t, peers, peerID)

		stopManager(t, manager)
	})

	t.Run("no peers from shrex.Sub and from discovery. Wait", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
		t.Cleanup(cancel)

		// create headerSub mock
		h := testHeader()
		headerSub := newSubLock(h)

		// start test manager
		manager := testManager(ctx, headerSub)

		// make sure peers are not returned before timeout
		timeoutCtx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
		t.Cleanup(cancel)
		_, _, err := manager.GetPeer(timeoutCtx, h.DataHash.Bytes())
		require.ErrorIs(t, err, context.DeadlineExceeded)

		peers := []peer.ID{"peer1", "peer2", "peer3"}

		// launch wait routine
		doneCh := make(chan struct{})
		go func() {
			defer close(doneCh)
			peerID, done, err := manager.GetPeer(ctx, h.DataHash.Bytes())
			done(ctx, ResultSuccess)
			require.NoError(t, err)
			require.Contains(t, peers, peerID)
		}()

		// send peers
		manager.fullNodes.add(peers...)

		// wait for peer to be received
		select {
		case <-doneCh:
		case <-ctx.Done():
			require.NoError(t, ctx.Err())
		}

		stopManager(t, manager)
	})

	t.Run("validated datahash by peers requested from shrex.Getter", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
		t.Cleanup(cancel)

		h := testHeader()
		headerSub := newSubLock(h, nil)

		// start test manager
		manager := testManager(ctx, headerSub)

		peers := []peer.ID{"peer1", "peer2", "peer3"}

		// spawn validator routine for every peer
		validationCh := make(chan pubsub.ValidationResult)
		for _, p := range peers {
			go func(peerID peer.ID) {
				select {
				case validationCh <- manager.Validate(ctx, peerID, h.DataHash.Bytes()):
				case <-ctx.Done():
					return
				}
			}(p)
		}

		// wait for peers to be collected in pool
		p := manager.getOrCreatePool(h.DataHash.String())
		waitPoolHasItems(ctx, t, p, 3)

		// mark pool as synced by calling GetPeer
		peerID, done, err := manager.GetPeer(ctx, h.DataHash.Bytes())
		done(ctx, ResultSuccess)
		require.NoError(t, err)
		require.Contains(t, peers, peerID)

		// wait for validators to finish
		for i := 0; i < len(peers); i++ {
			select {
			case result := <-validationCh:
				require.Equal(t, pubsub.ValidationIgnore, result)
			case <-ctx.Done():
				require.NoError(t, ctx.Err())
			}
		}

		stopManager(t, manager)
	})
}

func TestIntagration(t *testing.T) {
	t.Run("get peer from shrexsub", func(t *testing.T) {
		nw, err := mocknet.FullMeshConnected(3)
		require.NoError(t, err)
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
		t.Cleanup(cancel)

		bnPubSub, err := shrexsub.NewPubSub(ctx, nw.Hosts()[0], "test")
		require.NoError(t, err)

		fnPubSub1, err := shrexsub.NewPubSub(ctx, nw.Hosts()[1], "test")
		require.NoError(t, err)

		fnPubSub2, err := shrexsub.NewPubSub(ctx, nw.Hosts()[2], "test")
		require.NoError(t, err)

		require.NoError(t, bnPubSub.Start(ctx))
		require.NoError(t, fnPubSub1.Start(ctx))
		require.NoError(t, fnPubSub2.Start(ctx))

		fnPeerManager1 := testManager(ctx, newSubLock())
		fnPeerManager1.broadcast = fnPubSub1.Broadcast
		fnPeerManager1.ownPeerID = nw.Hosts()[1].ID()
		fnPeerManager2 := testManager(ctx, newSubLock())
		fnPeerManager2.broadcast = fnPubSub2.Broadcast

		require.NoError(t, fnPubSub1.AddValidator(fnPeerManager1.Validate))
		_, err = fnPubSub1.Subscribe()
		require.NoError(t, err)

		require.NoError(t, fnPubSub2.AddValidator(fnPeerManager2.Validate))
		_, err = fnPubSub2.Subscribe()
		require.NoError(t, err)

		// broadcast from BN
		peerHash := share.DataHash("peer1")
		require.NoError(t, bnPubSub.Broadcast(ctx, peerHash))

		// first FN should get message
		peerID, doneFn, err := fnPeerManager1.GetPeer(ctx, peerHash)
		require.NoError(t, err)

		// check that peerID matched bridge node
		require.Equal(t, nw.Hosts()[0].ID(), peerID)
		// release the validator by using ResultSuccess, it should allow message retransmit
		doneFn(ctx, ResultSuccess)

		// check for retransmitted message to arrive to second FN
		peerID2, doneFn2, err := fnPeerManager2.GetPeer(ctx, peerHash)
		if peerID2 == nw.Hosts()[0].ID() {
			// if got bridge node peer id, remove it from pool and retry
			doneFn2(ctx, ResultPeerMisbehaved)
			peerID2, doneFn2, err = fnPeerManager2.GetPeer(ctx, peerHash)
		}
		require.NoError(t, err)
		// should be first FN peer id
		require.Equal(t, nw.Hosts()[1].ID(), peerID2)
	})

	t.Run("get peer from discovery", func(t *testing.T) {
		nw, err := mocknet.FullMeshConnected(3)
		require.NoError(t, err)
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
		t.Cleanup(cancel)

		bs := host.InfoFromHost(nw.Hosts()[2])
		opts := []dht.Option{
			dht.Mode(dht.ModeAuto),
			dht.BootstrapPeers(*bs),
			//dht.ProtocolPrefix("prefix"),
			//dht.Datastore(params.DataStore),
			dht.RoutingTableRefreshPeriod(time.Second),
		}

		bsOpts := append(opts,
			dht.Mode(dht.ModeServer), // it must accept incoming connections
			dht.BootstrapPeers(),     // no bootstrappers for a bootstrapper ¯\_(ツ)_/¯
		)
		bsRouter, err := dht.New(ctx, nw.Hosts()[2], bsOpts...)
		require.NoError(t, err)
		require.NoError(t, bsRouter.Bootstrap(ctx))

		router1, err := dht.New(ctx, nw.Hosts()[0], opts...)
		require.NoError(t, err)
		bnDisc := discovery.NewDiscovery(
			nw.Hosts()[0],
			routingdisc.NewRoutingDiscovery(router1),
			10,
			time.Second,
			time.Second)

		router2, err := dht.New(ctx, nw.Hosts()[1], opts...)
		require.NoError(t, err)
		fnDisc := discovery.NewDiscovery(
			nw.Hosts()[1],
			routingdisc.NewRoutingDiscovery(router2),
			10,
			time.Second,
			time.Second)

		fnDisc.WithOnPeersUpdate(func(peerID peer.ID, isAdded bool) {
			fmt.Println("GOT peer", peerID.String(), isAdded)
		})

		require.NoError(t, router1.Bootstrap(ctx))
		require.NoError(t, router2.Bootstrap(ctx))
		go fnDisc.EnsurePeers(ctx)

		go bnDisc.Advertise(ctx)
		select {
		case <-ctx.Done():
			peers, err := fnDisc.Peers(ctx)
			require.NoError(t, err)
			fmt.Println("done", len(peers))
		}
	})
}

func testManager(ctx context.Context, headerSub libhead.Subscriber[*header.ExtendedHeader]) *Manager {
	ctx, cancel := context.WithCancel(ctx)
	manager := &Manager{
		headerSub: headerSub,
		pools:     make(map[string]*syncPool),
		broadcast: func(ctx context.Context, hash share.DataHash) error {
			return nil
		},
		poolSyncTimeout: 60 * time.Second,
		fullNodes:       newPool(),
		cancel:          cancel,
		done:            make(chan struct{}),
	}

	sub, _ := headerSub.Subscribe()
	go manager.subscribeHeader(ctx, sub)
	return manager
}

func stopManager(t *testing.T, m *Manager) {
	closeCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	t.Cleanup(cancel)
	require.NoError(t, m.Stop(closeCtx))
}

func testHeader() *header.ExtendedHeader {
	return &header.ExtendedHeader{
		RawHeader: header.RawHeader{},
	}
}

func waitPoolHasItems(ctx context.Context, t *testing.T, p *syncPool, count int) {
	for {
		p.pool.m.Lock()
		if p.pool.activeCount >= count {
			p.pool.m.Unlock()
			break
		}
		p.pool.m.Unlock()
		select {
		case <-ctx.Done():
			require.NoError(t, ctx.Err())
		default:
			time.Sleep(time.Millisecond)
		}
	}
}

type subLock struct {
	next     chan struct{}
	called   chan struct{}
	expected []*header.ExtendedHeader
}

func (s subLock) wait(ctx context.Context, count int) error {
	for i := 0; i < count; i++ {
		select {
		case <-s.called:
			err := s.release(ctx)
			if err != nil {
				return err
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}

func (s subLock) release(ctx context.Context) error {
	select {
	case s.next <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func newSubLock(expected ...*header.ExtendedHeader) *subLock {
	return &subLock{
		next:     make(chan struct{}),
		called:   make(chan struct{}),
		expected: expected,
	}
}

func (s *subLock) Subscribe() (libhead.Subscription[*header.ExtendedHeader], error) {
	return s, nil
}

func (s *subLock) AddValidator(f func(context.Context, *header.ExtendedHeader) pubsub.ValidationResult) error {
	panic("implement me")
}

func (s *subLock) NextHeader(ctx context.Context) (*header.ExtendedHeader, error) {
	close(s.called)

	// wait for call to be unlocked by release
	select {
	case <-s.next:
		h := s.expected[0]
		s.expected = s.expected[1:]
		s.called = make(chan struct{})
		return h, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (s *subLock) Cancel() {
}
