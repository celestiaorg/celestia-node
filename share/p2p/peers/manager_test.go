package peers

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/ipfs/go-datastore"
	dssync "github.com/ipfs/go-datastore/sync"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	routingdisc "github.com/libp2p/go-libp2p/p2p/discovery/routing"
	"github.com/libp2p/go-libp2p/p2p/net/conngater"
	mocknet "github.com/libp2p/go-libp2p/p2p/net/mock"
	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/libs/rand"

	libhead "github.com/celestiaorg/go-header"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/p2p/discovery"
	"github.com/celestiaorg/celestia-node/share/p2p/shrexsub"
)

func TestManager(t *testing.T) {
	t.Run("Validate pool by headerSub", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
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

	t.Run("Validate pool by shrex.Getter", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
		t.Cleanup(cancel)

		h := testHeader()
		headerSub := newSubLock(h, nil)

		// start test manager
		manager, err := testManager(ctx, headerSub)
		require.NoError(t, err)

		peerID, msg := peer.ID("peer1"), newShrexSubMsg(h)
		result := manager.Validate(ctx, peerID, msg)
		require.Equal(t, pubsub.ValidationIgnore, result)

		pID, _, err := manager.Peer(ctx, h.DataHash.Bytes(), h.Height())
		require.NoError(t, err)
		require.Equal(t, peerID, pID)

		// check pool validation
		require.True(t, manager.getPool(h.DataHash.String()).isValidatedDataHash.Load())
	})

	t.Run("validator", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
		t.Cleanup(cancel)

		// create headerSub mock
		h := testHeader()
		headerSub := newSubLock(h, nil)

		// start test manager
		manager, err := testManager(ctx, headerSub)
		require.NoError(t, err)

		// own messages should be be accepted
		msg := newShrexSubMsg(h)
		result := manager.Validate(ctx, manager.host.ID(), msg)
		require.Equal(t, pubsub.ValidationAccept, result)

		// normal messages should be ignored
		peerID := peer.ID("peer1")
		result = manager.Validate(ctx, peerID, msg)
		require.Equal(t, pubsub.ValidationIgnore, result)

		// mark peer as misbehaved to blacklist it
		pID, done, err := manager.Peer(ctx, h.DataHash.Bytes(), h.Height())
		require.NoError(t, err)
		require.Equal(t, peerID, pID)
		manager.params.EnableBlackListing = true
		done(ResultBlacklistPeer)

		// new messages from misbehaved peer should be Rejected
		result = manager.Validate(ctx, pID, msg)
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
		require.NoError(t, headerSub.wait(ctx, 1))

		// set syncTimeout to 0 to allow cleanup to find outdated datahash
		manager.params.PoolValidationTimeout = 0

		// create unvalidated pool
		peerID := peer.ID("peer1")
		msg := shrexsub.Notification{
			DataHash: share.DataHash("datahash1datahash1datahash1datahash1datahash1"),
			Height:   2,
		}
		manager.Validate(ctx, peerID, msg)

		// create validated pool
		validDataHash := share.DataHash("datahash2")
		manager.nodes.add("full")                    // add FN to unblock Peer call
		manager.Peer(ctx, validDataHash, h.Height()) //nolint:errcheck
		require.Len(t, manager.pools, 3)

		// trigger cleanup
		blacklisted := manager.cleanUp()
		require.Contains(t, blacklisted, peerID)
		require.Len(t, manager.pools, 2)

		// messages with blacklisted hash should be rejected right away
		peerID2 := peer.ID("peer2")
		result := manager.Validate(ctx, peerID2, msg)
		require.Equal(t, pubsub.ValidationReject, result)

		// check blacklisted pools
		require.True(t, manager.isBlacklistedHash(msg.DataHash))
		require.False(t, manager.isBlacklistedHash(validDataHash))
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
		manager.nodes.add(peers...)

		peerID, _, err := manager.Peer(ctx, h.DataHash.Bytes(), h.Height())
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
		_, _, err = manager.Peer(timeoutCtx, h.DataHash.Bytes(), h.Height())
		require.ErrorIs(t, err, context.DeadlineExceeded)

		peers := []peer.ID{"peer1", "peer2", "peer3"}

		// launch wait routine
		doneCh := make(chan struct{})
		go func() {
			defer close(doneCh)
			peerID, _, err := manager.Peer(ctx, h.DataHash.Bytes(), h.Height())
			require.NoError(t, err)
			require.Contains(t, peers, peerID)
		}()

		// send peers
		manager.nodes.add(peers...)

		// wait for peer to be received
		select {
		case <-doneCh:
		case <-ctx.Done():
			require.NoError(t, ctx.Err())
		}

		stopManager(t, manager)
	})

	t.Run("shrexSub sends a message lower than first headerSub header height, headerSub first", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
		t.Cleanup(cancel)

		h := testHeader()
		h.RawHeader.Height = 100
		headerSub := newSubLock(h, nil)

		// start test manager
		manager, err := testManager(ctx, headerSub)
		require.NoError(t, err)

		// unlock headerSub to read first header
		require.NoError(t, headerSub.wait(ctx, 1))
		// pool will be created for first headerSub header datahash
		require.Len(t, manager.pools, 1)

		// create shrexSub msg with height lower than first header from headerSub
		msg := shrexsub.Notification{
			DataHash: share.DataHash("datahash"),
			Height:   h.Height() - 1,
		}
		result := manager.Validate(ctx, "peer", msg)
		require.Equal(t, pubsub.ValidationIgnore, result)
		// pool will be created for first shrexSub message
		require.Len(t, manager.pools, 2)

		blacklisted := manager.cleanUp()
		require.Empty(t, blacklisted)
		// trigger cleanup and outdated pool should be removed
		require.Len(t, manager.pools, 1)
	})

	t.Run("shrexSub sends a message lower than first headerSub header height, shrexSub first", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
		t.Cleanup(cancel)

		h := testHeader()
		h.RawHeader.Height = 100
		headerSub := newSubLock(h, nil)

		// start test manager
		manager, err := testManager(ctx, headerSub)
		require.NoError(t, err)

		// create shrexSub msg with height lower than first header from headerSub
		msg := shrexsub.Notification{
			DataHash: share.DataHash("datahash"),
			Height:   h.Height() - 1,
		}
		result := manager.Validate(ctx, "peer", msg)
		require.Equal(t, pubsub.ValidationIgnore, result)

		// pool will be created for first shrexSub message
		require.Len(t, manager.pools, 1)

		// unlock headerSub to allow it to send next message
		require.NoError(t, headerSub.wait(ctx, 1))
		// second pool should be created
		require.Len(t, manager.pools, 2)

		// trigger cleanup and outdated pool should be removed
		blacklisted := manager.cleanUp()
		require.Len(t, manager.pools, 1)

		// check that no peers or hashes were blacklisted
		manager.params.PoolValidationTimeout = 0
		require.Len(t, blacklisted, 0)
		require.Len(t, manager.blacklistedHashes, 0)
	})

	t.Run("pools store window", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
		t.Cleanup(cancel)

		h := testHeader()
		h.RawHeader.Height = storedPoolsAmount * 2
		headerSub := newSubLock(h, nil)

		// start test manager
		manager, err := testManager(ctx, headerSub)
		require.NoError(t, err)

		// unlock headerSub to read first header
		require.NoError(t, headerSub.wait(ctx, 1))
		// pool will be created for first headerSub header datahash
		require.Len(t, manager.pools, 1)

		// create shrexSub msg with height lower than storedPoolsAmount
		msg := shrexsub.Notification{
			DataHash: share.DataHash("datahash"),
			Height:   h.Height() - storedPoolsAmount - 3,
		}
		result := manager.Validate(ctx, "peer", msg)
		require.Equal(t, pubsub.ValidationIgnore, result)

		// shrexSub message should be discarded and amount of pools should not change
		require.Len(t, manager.pools, 1)
	})
}

func TestIntegration(t *testing.T) {
	t.Run("get peer from shrexsub", func(t *testing.T) {
		nw, err := mocknet.FullMeshLinked(2)
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

		require.NoError(t, fnPubSub.AddValidator(fnPeerManager.Validate))
		_, err = fnPubSub.Subscribe()
		require.NoError(t, err)

		time.Sleep(time.Millisecond * 100)
		require.NoError(t, nw.ConnectAllButSelf())
		time.Sleep(time.Millisecond * 100)

		// broadcast from BN
		randHash := rand.Bytes(32)
		require.NoError(t, bnPubSub.Broadcast(ctx, shrexsub.Notification{
			DataHash: randHash,
			Height:   1,
		}))

		// FN should get message
		gotPeer, _, err := fnPeerManager.Peer(ctx, randHash, 13)
		require.NoError(t, err)

		// check that gotPeer matched bridge node
		require.Equal(t, nw.Hosts()[0].ID(), gotPeer)
	})

	t.Run("get peer from discovery", func(t *testing.T) {
		fullNodesTag := "fullNodes"
		nw, err := mocknet.FullMeshConnected(3)
		require.NoError(t, err)
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
		t.Cleanup(cancel)

		// set up bootstrapper
		bsHost := nw.Hosts()[0]
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
		bnHost := nw.Hosts()[1]
		bnRouter, err := dht.New(ctx, bnHost, opts...)
		require.NoError(t, err)

		params := discovery.DefaultParameters()
		params.AdvertiseInterval = time.Second

		bnDisc, err := discovery.NewDiscovery(
			params,
			bnHost,
			routingdisc.NewRoutingDiscovery(bnRouter),
			fullNodesTag,
		)
		require.NoError(t, err)

		// set up full node / receiver node
		fnHost := nw.Hosts()[2]
		fnRouter, err := dht.New(ctx, fnHost, opts...)
		require.NoError(t, err)

		// init peer manager for full node
		connGater, err := conngater.NewBasicConnectionGater(dssync.MutexWrap(datastore.NewMapDatastore()))
		require.NoError(t, err)
		fnPeerManager, err := NewManager(
			DefaultParameters(),
			nil,
			connGater,
		)
		require.NoError(t, err)

		waitCh := make(chan struct{})
		checkDiscoveredPeer := func(peerID peer.ID, isAdded bool) {
			defer close(waitCh)
			// check that obtained peer id is BN
			require.Equal(t, bnHost.ID(), peerID)
		}

		// set up discovery for full node with hook to peer manager and check discovered peer
		params = discovery.DefaultParameters()
		params.AdvertiseInterval = time.Second
		params.PeersLimit = 10

		fnDisc, err := discovery.NewDiscovery(
			params,
			fnHost,
			routingdisc.NewRoutingDiscovery(fnRouter),
			fullNodesTag,
			discovery.WithOnPeersUpdate(fnPeerManager.UpdateNodePool),
			discovery.WithOnPeersUpdate(checkDiscoveredPeer),
		)
		require.NoError(t, fnDisc.Start(ctx))
		t.Cleanup(func() {
			err = fnDisc.Stop(ctx)
			require.NoError(t, err)
		})

		require.NoError(t, bnRouter.Bootstrap(ctx))
		require.NoError(t, fnRouter.Bootstrap(ctx))

		go bnDisc.Advertise(ctx)

		select {
		case <-waitCh:
			require.Contains(t, fnPeerManager.nodes.peersList, bnHost.ID())
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

	connGater, err := conngater.NewBasicConnectionGater(dssync.MutexWrap(datastore.NewMapDatastore()))
	if err != nil {
		return nil, err
	}
	manager, err := NewManager(
		DefaultParameters(),
		host,
		connGater,
		WithShrexSubPools(shrexSub, headerSub),
	)
	if err != nil {
		return nil, err
	}
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
		RawHeader: header.RawHeader{
			Height:   1,
			DataHash: rand.Bytes(32),
		},
	}
}

type subLock struct {
	next     chan struct{}
	wg       *sync.WaitGroup
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
	wg := &sync.WaitGroup{}
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

func (s *subLock) SetVerifier(func(context.Context, *header.ExtendedHeader) error) error {
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

func newShrexSubMsg(h *header.ExtendedHeader) shrexsub.Notification {
	return shrexsub.Notification{
		DataHash: h.DataHash.Bytes(),
		Height:   h.Height(),
	}
}
