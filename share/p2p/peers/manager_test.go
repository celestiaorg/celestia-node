package peers

import (
	"context"
	sync2 "sync"
	"testing"
	"time"

	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/sync"
	routinghelpers "github.com/libp2p/go-libp2p-routing-helpers"
	"github.com/libp2p/go-libp2p/p2p/net/conngater"

	dht "github.com/libp2p/go-libp2p-kad-dht"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	routingdisc "github.com/libp2p/go-libp2p/p2p/discovery/routing"
	mocknet "github.com/libp2p/go-libp2p/p2p/net/mock"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/header"
	libhead "github.com/celestiaorg/celestia-node/libs/header"
	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/availability/discovery"
	"github.com/celestiaorg/celestia-node/share/p2p/shrexsub"
)

// TODO: add broadcast to tests
func TestManager(t *testing.T) {
	t.Run("validate datahash by headerSub", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*50)
		t.Cleanup(cancel)

		// create headerSub mock
		h := testHeader()
		headerSub := newSubLock(h, nil)

		// start test manager
		manager, err := testManager(ctx, headerSub)
		require.NoError(t, err)

		// wait until header is requested from header sub
		err = headerSub.wait(ctx, 1)
		require.NoError(t, err)

		// check validation
		require.True(t, manager.pools[h.DataHash.String()].isValidatedDataHash.Load())
		stopManager(t, manager)
	})

	t.Run("validate datahash by shrex.Getter", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
		t.Cleanup(cancel)

		h := testHeader()
		headerSub := newSubLock(h, nil)

		// start test manager
		manager, err := testManager(ctx, headerSub)
		require.NoError(t, err)

		peerID := peer.ID("peer1")
		result := manager.validate(ctx, peerID, h.DataHash.Bytes())
		require.Equal(t, pubsub.ValidationIgnore, result)

		pID, done, err := manager.Peer(ctx, h.DataHash.Bytes())
		require.NoError(t, err)
		require.Equal(t, peerID, pID)

		// check pool validation
		require.True(t, manager.pools[h.DataHash.String()].isValidatedDataHash.Load())

		done(ResultSuccess)
		// pool should be removed after success
		require.Len(t, manager.pools, 0)
	})

	t.Run("validator", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*50)
		t.Cleanup(cancel)

		// create headerSub mock
		h := testHeader()
		headerSub := newSubLock(h, nil)

		// start test manager
		manager, err := testManager(ctx, headerSub)
		require.NoError(t, err)

		result := manager.validate(ctx, manager.host.ID(), h.DataHash.Bytes())
		require.Equal(t, pubsub.ValidationAccept, result)

		peerID := peer.ID("peer1")
		result = manager.validate(ctx, peerID, h.DataHash.Bytes())
		require.Equal(t, pubsub.ValidationIgnore, result)

		pID, done, err := manager.Peer(ctx, h.DataHash.Bytes())
		require.NoError(t, err)
		require.Equal(t, peerID, pID)

		// mark peer as misbehaved tp blacklist it
		done(ResultPeerMisbehaved)

		// misbehaved should be Rejected
		result = manager.validate(ctx, pID, h.DataHash.Bytes())
		require.Equal(t, pubsub.ValidationReject, result)

		stopManager(t, manager)
	})

	t.Run("cleanup", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
		t.Cleanup(cancel)

		// create headerSub mock
		h := testHeader()
		headerSub := newSubLock(h)

		// start test manager
		manager, err := testManager(ctx, headerSub)
		require.NoError(t, err)

		peerID := peer.ID("peer1")
		manager.validate(ctx, peerID, h.DataHash.Bytes())
		// set syncTimeout to 0 to allow cleanup to find outdated datahash
		manager.poolSyncTimeout = 0

		blacklisted := manager.cleanUp()
		require.Contains(t, blacklisted, peerID)
		require.True(t, manager.hashIsBlacklisted(h.DataHash.Bytes()))
	})

	t.Run("no peers from shrex.Sub, get from discovery", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
		t.Cleanup(cancel)

		// create headerSub mock
		h := testHeader()
		headerSub := newSubLock(h)

		// start test manager
		manager, err := testManager(ctx, headerSub)
		require.NoError(t, err)

		// add peers to fullnodes, imitating discovery add
		peers := []peer.ID{"peer1", "peer2", "peer3"}
		manager.fullNodes.add(peers...)

		peerID, done, err := manager.Peer(ctx, h.DataHash.Bytes())
		done(ResultSuccess)
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
		manager, err := testManager(ctx, headerSub)
		require.NoError(t, err)

		// make sure peers are not returned before timeout
		timeoutCtx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
		t.Cleanup(cancel)
		_, _, err = manager.Peer(timeoutCtx, h.DataHash.Bytes())
		require.ErrorIs(t, err, context.DeadlineExceeded)

		peers := []peer.ID{"peer1", "peer2", "peer3"}

		// launch wait routine
		doneCh := make(chan struct{})
		go func() {
			defer close(doneCh)
			peerID, done, err := manager.Peer(ctx, h.DataHash.Bytes())
			done(ResultSuccess)
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

	t.Run("get peer from discovery", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
		t.Cleanup(cancel)

		h := testHeader()
		headerSub := newSubLock(h, nil)

		// start test manager
		manager, err := testManager(ctx, headerSub)
		require.NoError(t, err)

		peerID := peer.ID("peer1")
		result := manager.validate(ctx, peerID, h.DataHash.Bytes())
		require.Equal(t, pubsub.ValidationIgnore, result)

		pID, done, err := manager.Peer(ctx, h.DataHash.Bytes())
		require.NoError(t, err)
		require.Equal(t, peerID, pID)
		done(ResultSuccess)

		// check pool is soft deleted and marked synced
		pool := manager.getOrCreatePool(h.DataHash.String())
		require.Len(t, pool.peersList, 0)
		require.True(t, pool.isSynced.Load())

		// add peer on synced pool should be noop
		pool.add("peer1", "peer2")
		require.Len(t, pool.peersList, 0)
	})
}

func TestIntegration(t *testing.T) {
	t.Run("get peer from shrexsub", func(t *testing.T) {
		t.SkipNow()
		nw, err := mocknet.FullMeshConnected(3)
		require.NoError(t, err)
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
		t.Cleanup(cancel)

		bnPubSub, err := shrexsub.NewPubSub(ctx, nw.Hosts()[0], "test")
		require.NoError(t, err)

		fnPubSub, err := shrexsub.NewPubSub(ctx, nw.Hosts()[1], "test")
		require.NoError(t, err)

		require.NoError(t, bnPubSub.Start(ctx))
		require.NoError(t, fnPubSub.Start(ctx))

		fnPeerManager, err := testManager(ctx, newSubLock())
		require.NoError(t, err)
		fnPeerManager.host = nw.Hosts()[1]

		require.NoError(t, fnPubSub.AddValidator(fnPeerManager.validate))
		_, err = fnPubSub.Subscribe()
		require.NoError(t, err)

		// broadcast from BN
		peerHash := share.DataHash("peer1")
		require.NoError(t, bnPubSub.Broadcast(ctx, peerHash))

		// FN should get message
		peerID, _, err := fnPeerManager.Peer(ctx, peerHash)
		require.NoError(t, err)

		// check that peerID matched bridge node
		require.Equal(t, nw.Hosts()[0].ID(), peerID)
	})

	t.Run("get peer from discovery", func(t *testing.T) {
		nw, err := mocknet.FullMeshConnected(3)
		require.NoError(t, err)
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
		t.Cleanup(cancel)

		// set up bootstrapper
		bsHost := nw.Hosts()[2]
		bs := host.InfoFromHost(bsHost)
		opts := []dht.Option{
			dht.Mode(dht.ModeAuto),
			dht.BootstrapPeers(*bs),
			dht.RoutingTableRefreshPeriod(time.Second),
		}

		bsOpts := opts
		bsOpts = append(bsOpts,
			dht.Mode(dht.ModeServer), // it must accept incoming connections
			dht.BootstrapPeers(),     // no bootstrappers for a bootstrapper ¯\_(ツ)_/¯
		)
		bsRouter, err := dht.New(ctx, bsHost, bsOpts...)
		require.NoError(t, err)
		require.NoError(t, bsRouter.Bootstrap(ctx))

		// set up broadcaster node
		bnHost := nw.Hosts()[0]
		router1, err := dht.New(ctx, bnHost, opts...)
		require.NoError(t, err)
		bnDisc := discovery.NewDiscovery(
			nw.Hosts()[0],
			routingdisc.NewRoutingDiscovery(router1),
			10,
			time.Second,
			time.Second)

		// set up full node / receiver node
		fnHost := nw.Hosts()[0]
		router2, err := dht.New(ctx, fnHost, opts...)
		require.NoError(t, err)
		fnDisc := discovery.NewDiscovery(
			nw.Hosts()[1],
			routingdisc.NewRoutingDiscovery(router2),
			10,
			time.Second,
			time.Second)

		// hook peer manager to discovery
		connGater, err := conngater.NewBasicConnectionGater(sync.MutexWrap(datastore.NewMapDatastore()))
		require.NoError(t, err)
		fnPeerManager := NewManager(nil, nil, fnDisc, nil, connGater, time.Minute)

		waitCh := make(chan struct{})
		fnDisc.WithOnPeersUpdate(func(peerID peer.ID, isAdded bool) {
			defer close(waitCh)
			// check that obtained peer id is same as BN
			require.Equal(t, nw.Hosts()[0].ID(), peerID)
		})

		require.NoError(t, router1.Bootstrap(ctx))
		require.NoError(t, router2.Bootstrap(ctx))

		go fnDisc.EnsurePeers(ctx)
		go bnDisc.Advertise(ctx)

		select {
		case <-waitCh:
			require.Contains(t, fnPeerManager.fullNodes.peersList, fnHost.ID())
		case <-ctx.Done():
			require.NoError(t, ctx.Err())
		}
	})
}

func testManager(ctx context.Context, headerSub libhead.Subscriber[*header.ExtendedHeader]) (*Manager, error) {
	host, err := mocknet.New().GenPeer()
	if err != nil {
		return nil, err
	}
	shrexSub, err := shrexsub.NewPubSub(ctx, host, "test")
	if err != nil {
		return nil, err
	}
	disc := discovery.NewDiscovery(nil,
		routingdisc.NewRoutingDiscovery(routinghelpers.Null{}), 0, time.Second, time.Second)
	connGater, err := conngater.NewBasicConnectionGater(sync.MutexWrap(datastore.NewMapDatastore()))
	if err != nil {
		return nil, err
	}
	manager := NewManager(headerSub, shrexSub, disc, host, connGater, time.Minute)
	err = manager.Start(ctx)
	return manager, err
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

type subLock struct {
	next     chan struct{}
	wg       *sync2.WaitGroup
	expected []*header.ExtendedHeader
}

func (s subLock) wait(ctx context.Context, count int) error {
	s.wg.Add(count)
	for i := 0; i < count; i++ {
		err := s.release(ctx)
		if err != nil {
			return err
		}
	}
	s.wg.Wait()
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
	wg := &sync2.WaitGroup{}
	wg.Add(1)
	return &subLock{
		next:     make(chan struct{}),
		expected: expected,
		wg:       wg,
	}
}

func (s *subLock) Subscribe() (libhead.Subscription[*header.ExtendedHeader], error) {
	return s, nil
}

func (s *subLock) AddValidator(f func(context.Context, *header.ExtendedHeader) pubsub.ValidationResult) error {
	panic("implement me")
}

func (s *subLock) NextHeader(ctx context.Context) (*header.ExtendedHeader, error) {
	s.wg.Done()

	// wait for call to be unlocked by release
	select {
	case <-s.next:
		h := s.expected[0]
		s.expected = s.expected[1:]
		return h, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (s *subLock) Cancel() {
}
