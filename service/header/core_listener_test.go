package header

import (
	"context"
	"testing"

	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/sync"
	mdutils "github.com/ipfs/go-merkledag/test"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	mocknet "github.com/libp2p/go-libp2p/p2p/net/mock"
	"github.com/stretchr/testify/require"

	"github.com/tendermint/tendermint/types"

	"github.com/celestiaorg/celestia-node/core"
)

// TestCoreListener tests the lifecycle of the core listener.
func TestCoreListener(t *testing.T) {
	net, err := mocknet.FullMeshConnected(context.Background(), 2)
	require.NoError(t, err)
	// create pubsub for host
	ps1, err := pubsub.NewGossipSub(context.Background(), net.Hosts()[0],
		pubsub.WithMessageSignaturePolicy(pubsub.StrictNoSign))
	require.NoError(t, err)
	topic1, err := ps1.Join(PubSubTopic)
	require.NoError(t, err)
	// create pubsub for peer-side (to test broadcast comes through network)
	ps2, err := pubsub.NewGossipSub(context.Background(), net.Hosts()[1],
		pubsub.WithMessageSignaturePolicy(pubsub.StrictNoSign))
	require.NoError(t, err)
	topic2, err := ps2.Join(PubSubTopic)
	require.NoError(t, err)
	sub, err := topic2.Subscribe()
	require.NoError(t, err)

	md := mdutils.Mock()
	mockClient, fetcher := createCoreFetcher()
	t.Cleanup(func() {
		mockClient.Stop() // nolint:errcheck
	})
	ex := NewCoreExchange(fetcher, md)

	cl, err := NewCoreListener(ex, topic1)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	go cl.listen(ctx)

	// ensure headers are getting broadcasted to the gossipsub topic
	for i := 1; i <= 5; i++ {
		msg, err := sub.Next(context.Background())
		require.NoError(t, err)

		var resp ExtendedHeader
		err = resp.UnmarshalBinary(msg.Data)
		require.NoError(t, err)
		require.Equal(t, i, int(resp.Height))
	}

	require.NoError(t, cl.sub.Cancel())
}

// TestCoreHeaderService tests that the HeaderService properly
// kicks off a sync routine and subsequently, a CoreListener routine.
func TestCoreHeaderService(t *testing.T) {
	md := mdutils.Mock()
	// create a mockCore client that we can use later to generate fake blocks
	coreNode := core.StartMockNode()
	coreClient := core.NewEmbeddedFromNode(coreNode)
	t.Cleanup(func() {
		coreNode.Stop()   // nolint:errcheck
		coreClient.Stop() // nolint:errcheck
	})

	fetcher := core.NewBlockFetcher(coreClient)
	ex := NewCoreExchange(fetcher, md)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	// subscribe to read once from sub to generate a block, and unsubscribe
	sub, err := ex.fetcher.SubscribeNewBlockEvent(ctx)
	require.NoError(t, err)
	<-sub
	require.NoError(t, ex.fetcher.UnsubscribeNewBlockEvent(ctx))
	// get head
	head, err := ex.RequestHead(ctx)
	require.NoError(t, err)

	localStore, err := NewStoreWithHead(sync.MutexWrap(datastore.NewMapDatastore()), head)
	require.NoError(t, err)

	// create pubsub for host
	net, err := mocknet.FullMeshConnected(ctx, 2)
	require.NoError(t, err)
	ps1, err := pubsub.NewGossipSub(context.Background(), net.Hosts()[0],
		pubsub.WithMessageSignaturePolicy(pubsub.StrictNoSign))
	require.NoError(t, err)

	// create syncer and header service
	syncer := NewSyncer(ex, localStore, head.Hash())
	headerServ := NewHeaderService(syncer, ps1)

	// create pubsub for peer-side (to test broadcast comes through network)
	ps2, err := pubsub.NewGossipSub(context.Background(), net.Hosts()[1],
		pubsub.WithMessageSignaturePolicy(pubsub.StrictNoSign))
	require.NoError(t, err)
	topic2, err := ps2.Join(PubSubTopic)
	require.NoError(t, err)
	peerSub, err := topic2.Subscribe()
	require.NoError(t, err)

	// start header service
	err = headerServ.Start(ctx)
	require.NoError(t, err)

	// wait til syncing is finished to generate new blocks for the
	// listener to catch and broadcast as ExtendedHeaders
	<-syncer.done

	// create a new subscription to generate another new block
	blockGen := core.NewEmbeddedFromNode(coreNode)
	blockSub, err := blockGen.Subscribe(context.Background(), "TEST",
		types.QueryForEvent(types.EventNewBlock).String())
	require.NoError(t, err)
	// get expected block details
	rawExpected := <-blockSub
	expected, ok := rawExpected.Data.(types.EventDataNewBlock)
	if !ok {
		t.Fatalf("unexpected: %v", rawExpected)
	}
	err = blockGen.Unsubscribe(context.Background(), "TEST",
		types.QueryForEvent(types.EventNewBlock).String())
	require.NoError(t, err)

	// ensure expected block is broadcasted to gossipsub network
	msg, err := peerSub.Next(context.Background())
	require.NoError(t, err)

	// unmarshal gossipsub message into ExtendedHeader
	var nextHeader ExtendedHeader
	err = nextHeader.UnmarshalBinary(msg.Data)
	require.NoError(t, err)
	require.Equal(t, expected.Block.Height, nextHeader.Height)

	// stop header service
	require.NoError(t, headerServ.Stop(context.Background()))
}
