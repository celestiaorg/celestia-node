package core

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/cosmos/cosmos-sdk/client/flags"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/event"
	mocknet "github.com/libp2p/go-libp2p/p2p/net/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/go-header/p2p"

	"github.com/celestiaorg/celestia-node/header"
	nodep2p "github.com/celestiaorg/celestia-node/nodebuilder/p2p"
	"github.com/celestiaorg/celestia-node/share/eds"
	"github.com/celestiaorg/celestia-node/share/p2p/shrexsub"
)

const networkID = "private"

// TestListener tests the lifecycle of the core listener.
func TestListener(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	t.Cleanup(cancel)

	// create mocknet with two pubsub endpoints
	ps0, ps1 := createMocknetWithTwoPubsubEndpoints(ctx, t)
	subscriber := p2p.NewSubscriber[*header.ExtendedHeader](ps1, header.MsgID, networkID)
	err := subscriber.SetVerifier(func(context.Context, *header.ExtendedHeader) error {
		return nil
	})
	require.NoError(t, err)
	require.NoError(t, subscriber.Start(ctx))
	subs, err := subscriber.Subscribe()
	require.NoError(t, err)
	t.Cleanup(subs.Cancel)

	// create one block to store as Head in local store and then unsubscribe from block events
	fetcher, _ := createCoreFetcher(t, DefaultTestConfig())
	eds := createEdsPubSub(ctx, t)
	// create Listener and start listening
	cl := createListener(ctx, t, fetcher, ps0, eds, createStore(t))
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

// TestListenerWithNonEmptyBlocks ensures that non-empty blocks are actually
// stored to eds.Store.
func TestListenerWithNonEmptyBlocks(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	t.Cleanup(cancel)

	// create mocknet with two pubsub endpoints
	ps0, _ := createMocknetWithTwoPubsubEndpoints(ctx, t)

	// create one block to store as Head in local store and then unsubscribe from block events
	cfg := DefaultTestConfig()
	fetcher, cctx := createCoreFetcher(t, cfg)
	eds := createEdsPubSub(ctx, t)

	store := createStore(t)
	err := store.Start(ctx)
	require.NoError(t, err)
	t.Cleanup(func() {
		err = store.Stop(ctx)
		require.NoError(t, err)
	})

	// create Listener and start listening
	cl := createListener(ctx, t, fetcher, ps0, eds, store)
	err = cl.Start(ctx)
	require.NoError(t, err)

	// listen for eds hashes broadcasted through eds-sub and ensure store has
	// already stored them
	sub, err := eds.Subscribe()
	require.NoError(t, err)
	t.Cleanup(sub.Cancel)

	empty := header.EmptyDAH()
	// TODO extract 16
	for i := 0; i < 16; i++ {
		_, err := cctx.FillBlock(16, cfg.Accounts, flags.BroadcastBlock)
		require.NoError(t, err)
		msg, err := sub.Next(ctx)
		require.NoError(t, err)

		if bytes.Equal(empty.Hash(), msg.DataHash) {
			continue
		}

		has, err := store.Has(ctx, msg.DataHash)
		require.NoError(t, err)
		require.True(t, has)
	}

	err = cl.Stop(ctx)
	require.NoError(t, err)
	require.Nil(t, cl.cancel)
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
	store *eds.Store,
) *Listener {
	p2pSub := p2p.NewSubscriber[*header.ExtendedHeader](ps, header.MsgID, networkID)
	err := p2pSub.Start(ctx)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, p2pSub.Stop(ctx))
	})

	return NewListener(p2pSub, fetcher, edsSub.Broadcast, header.MakeExtendedHeader, store, nodep2p.BlockTime)
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
