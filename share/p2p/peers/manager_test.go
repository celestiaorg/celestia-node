package peers

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/header"
	header_mock "github.com/celestiaorg/celestia-node/header/mocks"
)

func TestManager(t *testing.T) {
	t.Run("validated datahash, header comes first", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
		t.Cleanup(cancel)

		// create headerSub mock
		h := testHeader()
		ctrl := gomock.NewController(t)
		headerSub := header_mock.NewMockSubscription(ctrl)
		headerSub.EXPECT().Cancel()
		nextHeader := waitNextHeader(headerSub, h, nil)

		// start test manager
		manager := testManager(headerSub)

		// wait until header is requested from header sub
		require.NoError(t, nextHeader.wait(ctx, 1))

		// validator should return accept, since header is synced
		peerID := peer.ID("peer")
		validation := manager.validate(ctx, peerID, h.DataHash.Bytes())
		require.Equal(t, pubsub.ValidationAccept, validation)

		p, err := manager.GetPeer(ctx, h.DataHash.String())
		require.NoError(t, err)
		require.Equal(t, peerID, p)

		stopManager(t, manager)
	})

	t.Run("validated datahash, peers comes first", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
		t.Cleanup(cancel)

		// create headerSub mock
		h := testHeader()
		ctrl := gomock.NewController(t)
		headerSub := header_mock.NewMockSubscription(ctrl)
		headerSub.EXPECT().Cancel()
		nextHeader := waitNextHeader(headerSub, h, nil)

		// start test manager
		manager := testManager(headerSub)

		peers := []peer.ID{"peer1", "peer2", "peer3"}

		// spawn wait routine, that will unlock when all validators have returned
		var wg sync.WaitGroup
		done := make(chan struct{})
		wg.Add(len(peers))
		go func() {
			defer close(done)
			wg.Wait()
		}()

		// spawn validator routine for every peer
		for _, p := range peers {
			go func(peerID peer.ID) {
				defer wg.Done()
				validation := manager.validate(ctx, peerID, h.DataHash.Bytes())
				require.Equal(t, pubsub.ValidationAccept, validation)
			}(p)
		}

		// wait for peers to be collected in pool
		p := manager.getOrCreateUnvalidatedPool(h.DataHash.Bytes())
		for {
			p.pool.m.Lock()
			count := p.pool.activeCount
			p.pool.m.Unlock()
			if count == 3 {
				break
			}
			select {
			case <-ctx.Done():
				require.NoError(t, ctx.Err())
			default:
			}
		}

		// release headerSub with validating header
		require.NoError(t, nextHeader.wait(ctx, 1))

		// wait for validators to finish
		select {
		case <-done:
		case <-ctx.Done():
			require.NoError(t, ctx.Err())
		}

		stopManager(t, manager)
	})

	t.Run("no datahash in timeout, reject validators", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
		t.Cleanup(cancel)

		// create headerSub mock
		h := testHeader()
		ctrl := gomock.NewController(t)
		headerSub := header_mock.NewMockSubscription(ctrl)
		headerSub.EXPECT().Cancel()
		waitNextHeader(headerSub, h)

		// start test manager
		manager := testManager(headerSub)

		peers := []peer.ID{"peer1", "peer2", "peer3"}

		// spawn wait routine, that will unlock when all validators have returned
		var wg sync.WaitGroup
		done := make(chan struct{})
		wg.Add(len(peers))
		go func() {
			defer close(done)
			wg.Wait()
		}()

		// set syncTimeout to 0 for fast rejects
		manager.poolSyncTimeout = 0

		// spawn validator routine for every peer and ensure rejects
		for _, p := range peers {
			go func(peerID peer.ID) {
				defer wg.Done()
				validation := manager.validate(ctx, peerID, h.DataHash.Bytes())
				require.Equal(t, pubsub.ValidationReject, validation)
			}(p)
		}

		// wait for validators to finish
		select {
		case <-done:
		case <-ctx.Done():
			require.NoError(t, ctx.Err())
		}

		stopManager(t, manager)
	})

	t.Run("no peers from shrex.Sub, get from discovery", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
		t.Cleanup(cancel)

		// create headerSub mock
		h := testHeader()
		ctrl := gomock.NewController(t)
		headerSub := header_mock.NewMockSubscription(ctrl)
		waitNextHeader(headerSub, h)
		headerSub.EXPECT().Cancel()

		// start test manager
		manager := testManager(headerSub)

		// add peers to fullnodes, imitating discovery add
		peers := []peer.ID{"peer1", "peer2", "peer3"}
		manager.fullNodes.add(peers...)

		peerID, err := manager.GetPeer(ctx, h.DataHash.String())
		require.NoError(t, err)
		require.Contains(t, peers, peerID)

		stopManager(t, manager)
	})

	t.Run("no peers from shrex.Sub and from discovery. Wait", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
		t.Cleanup(cancel)

		// create headerSub mock
		h := testHeader()
		ctrl := gomock.NewController(t)
		headerSub := header_mock.NewMockSubscription(ctrl)
		waitNextHeader(headerSub, h)
		headerSub.EXPECT().Cancel()

		// start test manager
		manager := testManager(headerSub)

		// make sure peers are not returned before timeout
		timeoutCtx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
		t.Cleanup(cancel)
		_, err := manager.GetPeer(timeoutCtx, h.DataHash.String())
		require.ErrorIs(t, err, context.DeadlineExceeded)

		peers := []peer.ID{"peer1", "peer2", "peer3"}
		done := make(chan struct{})
		go func() {
			defer close(done)
			peerID, err := manager.GetPeer(ctx, h.DataHash.String())
			require.NoError(t, err)
			require.Contains(t, peers, peerID)
		}()

		// send peers
		manager.fullNodes.add(peers...)

		// wait for peer to be received
		select {
		case <-done:
		case <-ctx.Done():
			require.NoError(t, ctx.Err())
		}

		stopManager(t, manager)
	})

	t.Run("validated datahash by peers requested from shrex.Getter", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
		t.Cleanup(cancel)

		h := testHeader()
		dataHash := h.DataHash

		ctrl := gomock.NewController(t)
		headerSub := header_mock.NewMockSubscription(ctrl)
		headerSub.EXPECT().Cancel()
		waitNextHeader(headerSub, h)

		// start test manager
		manager := testManager(headerSub)

		peers := []peer.ID{"peer1", "peer2", "peer3"}

		// spawn wait routine, that will unlock when all validators have returned
		var wg sync.WaitGroup
		done := make(chan struct{})
		wg.Add(len(peers))
		go func() {
			defer close(done)
			wg.Wait()
		}()

		// spawn validator routine for every peer
		for _, p := range peers {
			go func(peerID peer.ID) {
				defer wg.Done()
				validation := manager.validate(ctx, peerID, dataHash.Bytes())
				require.Equal(t, pubsub.ValidationAccept, validation)
			}(p)
		}

		// wait for peers to be collected in pool
		p := manager.getOrCreateUnvalidatedPool(dataHash.Bytes())
		for {
			p.pool.m.Lock()
			count := p.pool.activeCount
			p.pool.m.Unlock()
			if count == 3 {
				break
			}
			select {
			case <-ctx.Done():
				require.NoError(t, ctx.Err())
			default:
			}
		}

		// wait for first peers to be added to pool
		peerID, err := manager.GetPeer(ctx, dataHash.String())
		require.NoError(t, err)
		require.Contains(t, peers, peerID)

		// wait for validators to return
		select {
		case <-done:
		case <-ctx.Done():
			require.NoError(t, ctx.Err())
		}

		stopManager(t, manager)
	})
}

func testManager(headerSub *header_mock.MockSubscription) *Manager {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	manager := &Manager{
		headerSub:       headerSub,
		m:               new(sync.Mutex),
		pools:           make(map[hashStr]syncPool),
		poolSyncTimeout: 60 * time.Second,
		fullNodes:       newPool(),
		cancel:          cancel,
		done:            make(chan struct{}),
	}

	go manager.subscribeHeader(ctx)
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

type subLock struct {
	next   chan struct{}
	called chan struct{}
}

func (n subLock) wait(ctx context.Context, count int) error {
	for i := 0; i < count; i++ {
		select {
		case <-n.called:
			err := n.release(ctx)
			if err != nil {
				return err
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}

func (n subLock) release(ctx context.Context) error {
	select {
	case n.next <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func waitNextHeader(subscription *header_mock.MockSubscription, expected ...*header.ExtendedHeader) subLock {
	n := subLock{
		next:   make(chan struct{}),
		called: make(chan struct{}),
	}

	for i := range expected {
		i := i
		subscription.EXPECT().NextHeader(gomock.Any()).DoAndReturn(func(ctx context.Context) (*header.ExtendedHeader, error) {
			close(n.called)
			defer func() {
				n.called = make(chan struct{})
			}()

			// wait for
			select {
			case <-n.next:
				return expected[i], nil
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		})
	}
	return n
}
