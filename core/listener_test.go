package core

import (
	"context"
	"testing"
	"time"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/event"
	mocknet "github.com/libp2p/go-libp2p/p2p/net/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/go-header/p2p"

	"github.com/celestiaorg/celestia-node/header"
	nodep2p "github.com/celestiaorg/celestia-node/nodebuilder/p2p"
	"github.com/celestiaorg/celestia-node/pruner"
	"github.com/celestiaorg/celestia-node/share/shwap/p2p/shrex/shrexsub"
	"github.com/celestiaorg/celestia-node/store"
)

const testChainID = "private"

// TestListener tests the lifecycle of the core listener.
func TestListener(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	t.Cleanup(cancel)

	// create mocknet with two pubsub endpoints
	ps0, ps1 := createMocknetWithTwoPubsubEndpoints(ctx, t)
	subscriber, err := p2p.NewSubscriber[*header.ExtendedHeader](
		ps1,
		header.MsgID,
		p2p.WithSubscriberNetworkID(testChainID),
	)
	require.NoError(t, err)
	err = subscriber.SetVerifier(func(context.Context, *header.ExtendedHeader) error {
		return nil
	})
	require.NoError(t, err)
	require.NoError(t, subscriber.Start(ctx))
	subs, err := subscriber.Subscribe()
	require.NoError(t, err)
	t.Cleanup(subs.Cancel)

	// create one block to store as Head in local store and then unsubscribe from block events
	cfg := DefaultTestConfig()
	cfg.Genesis.ChainID = testChainID
	fetcher, _ := createCoreFetcher(t, cfg)

	eds := createEdsPubSub(ctx, t)

	// create Listener and start listening
	store, err := store.NewStore(store.DefaultParameters(), t.TempDir())
	require.NoError(t, err)
	cl := createListener(ctx, t, fetcher, ps0, eds, store, testChainID)
	err = cl.Start(ctx)
	require.NoError(t, err)

	edsSubs, err := eds.Subscribe()
	require.NoError(t, err)
	t.Cleanup(edsSubs.Cancel)

	// ensure headers and dataHash are getting broadcasted to the relevant topics
	for i := 0; i < 5; i++ {
		_, err := subs.NextHeader(ctx)
		require.NoError(t, err)
	}

	err = cl.Stop(ctx)
	require.NoError(t, err)
	require.Nil(t, cl.cancel)
}

func TestListenerWithWrongChainRPC(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	t.Cleanup(cancel)

	// create mocknet with two pubsub endpoints
	ps0, _ := createMocknetWithTwoPubsubEndpoints(ctx, t)

	// create one block to store as Head in local store and then unsubscribe from block events
	cfg := DefaultTestConfig()
	cfg.Genesis.ChainID = testChainID
	fetcher, _ := createCoreFetcher(t, cfg)
	eds := createEdsPubSub(ctx, t)

	store, err := store.NewStore(store.DefaultParameters(), t.TempDir())
	require.NoError(t, err)

	// create Listener and start listening
	cl := createListener(ctx, t, fetcher, ps0, eds, store, "wrong-chain-rpc")
	sub, err := cl.fetcher.SubscribeNewBlockEvent(ctx)
	require.NoError(t, err)

	err = cl.listen(ctx, sub)
	assert.ErrorIs(t, err, errInvalidSubscription)
}

// TestListener_DoesNotStoreHistoric tests the (unlikely) case that
// blocks come through the listener's subscription that are actually
// older than the sampling window.
func TestListener_DoesNotStoreHistoric(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	t.Cleanup(cancel)

	// create mocknet with two pubsub endpoints
	ps0, _ := createMocknetWithTwoPubsubEndpoints(ctx, t)

	// create one block to store as Head in local store and then unsubscribe from block events
	cfg := DefaultTestConfig()
	cfg.Genesis.ChainID = testChainID
	fetcher, cctx := createCoreFetcher(t, cfg)
	eds := createEdsPubSub(ctx, t)

	store, err := store.NewStore(store.DefaultParameters(), t.TempDir())
	require.NoError(t, err)

	// create Listener and start listening
	opt := WithAvailabilityWindow(pruner.AvailabilityWindow(time.Nanosecond))
	cl := createListener(ctx, t, fetcher, ps0, eds, store, testChainID, opt)

	dataRoots := generateNonEmptyBlocks(t, ctx, fetcher, cfg, cctx)

	err = cl.Start(ctx)
	require.NoError(t, err)

	// ensure none of the EDSes were stored
	for _, hash := range dataRoots {
		has, err := store.HasByHash(ctx, hash)
		require.NoError(t, err)
		assert.False(t, has)
	}
}

func createMocknetWithTwoPubsubEndpoints(ctx context.Context, t *testing.T) (*pubsub.PubSub, *pubsub.PubSub) {
	net, err := mocknet.FullMeshLinked(2)
	require.NoError(t, err)
	host0, host1 := net.Hosts()[0], net.Hosts()[1]

	// create pubsub for host
	ps0, err := pubsub.NewGossipSub(context.Background(), host0,
		pubsub.WithMessageSignaturePolicy(pubsub.StrictNoSign))
	require.NoError(t, err)
	// create pubsub for peer-side (to test broadcast comes through network)
	ps1, err := pubsub.NewGossipSub(context.Background(), host1,
		pubsub.WithMessageSignaturePolicy(pubsub.StrictNoSign))
	require.NoError(t, err)

	sub0, err := host0.EventBus().Subscribe(&event.EvtPeerIdentificationCompleted{})
	require.NoError(t, err)
	sub1, err := host1.EventBus().Subscribe(&event.EvtPeerIdentificationCompleted{})
	require.NoError(t, err)

	err = net.ConnectAllButSelf()
	require.NoError(t, err)

	// wait on both peer identification events
	for i := 0; i < 2; i++ {
		select {
		case <-sub0.Out():
		case <-sub1.Out():
		case <-ctx.Done():
			assert.FailNow(t, "timeout waiting for peers to connect")
		}
	}

	return ps0, ps1
}

func createListener(
	ctx context.Context,
	t *testing.T,
	fetcher *BlockFetcher,
	ps *pubsub.PubSub,
	edsSub *shrexsub.PubSub,
	store *store.Store,
	chainID string,
	opts ...Option,
) *Listener {
	p2pSub, err := p2p.NewSubscriber[*header.ExtendedHeader](ps, header.MsgID, p2p.WithSubscriberNetworkID(testChainID))
	require.NoError(t, err)

	err = p2pSub.Start(ctx)
	require.NoError(t, err)
	err = p2pSub.SetVerifier(func(ctx context.Context, msg *header.ExtendedHeader) error {
		return nil
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, p2pSub.Stop(ctx))
	})

	listener, err := NewListener(p2pSub, fetcher, edsSub.Broadcast, header.MakeExtendedHeader,
		store, nodep2p.BlockTime, append(opts, WithChainID(nodep2p.Network(chainID)))...)
	require.NoError(t, err)
	return listener
}

func createEdsPubSub(ctx context.Context, t *testing.T) *shrexsub.PubSub {
	net, err := mocknet.FullMeshLinked(1)
	require.NoError(t, err)
	edsSub, err := shrexsub.NewPubSub(ctx, net.Hosts()[0], "eds-test")
	require.NoError(t, err)
	require.NoError(t, edsSub.Start(ctx))
	t.Cleanup(func() {
		require.NoError(t, edsSub.Stop(ctx))
	})
	return edsSub
}
