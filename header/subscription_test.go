package header

import (
	"context"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p-core/event"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	mocknet "github.com/libp2p/go-libp2p/p2p/net/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSubscriber tests the header Service's implementation of Subscriber.
func TestSubscriber(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*15)
	defer cancel()

	// create mock network
	net, err := mocknet.FullMeshLinked(ctx, 2)
	require.NoError(t, err)

	suite := NewTestSuite(t, 3)

	// get mock host and create new gossipsub on it
	pubsub1, err := pubsub.NewGossipSub(ctx, net.Hosts()[0], pubsub.WithMessageSignaturePolicy(pubsub.StrictNoSign))
	require.NoError(t, err)

	// create sub-service lifecycles for header service 1
	p2pSub1 := NewP2PSubscriber(pubsub1)
	err = p2pSub1.Start(context.Background())
	require.NoError(t, err)

	// get mock host and create new gossipsub on it
	pubsub2, err := pubsub.NewGossipSub(ctx, net.Hosts()[1],
		pubsub.WithMessageSignaturePolicy(pubsub.StrictNoSign))
	require.NoError(t, err)

	// create sub-service lifecycles for header service 2
	p2pSub2 := NewP2PSubscriber(pubsub2)
	err = p2pSub2.Start(context.Background())
	require.NoError(t, err)

	sub0, err := net.Hosts()[0].EventBus().Subscribe(&event.EvtPeerIdentificationCompleted{})
	require.NoError(t, err)
	sub1, err := net.Hosts()[1].EventBus().Subscribe(&event.EvtPeerIdentificationCompleted{})
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

	// subscribe
	_, err = p2pSub2.Subscribe()
	require.NoError(t, err)

	subscription, err := p2pSub1.Subscribe()
	require.NoError(t, err)

	expectedHeader := suite.GenExtendedHeaders(1)[0]
	bin, err := expectedHeader.MarshalBinary()
	require.NoError(t, err)

	err = p2pSub2.topic.Publish(ctx, bin, pubsub.WithReadiness(pubsub.MinTopicSize(1)))
	require.NoError(t, err)

	// get next ExtendedHeader from network
	header, err := subscription.NextHeader(ctx)
	require.NoError(t, err)

	assert.Equal(t, expectedHeader.Height, header.Height)
	assert.Equal(t, expectedHeader.Hash(), header.Hash())
	assert.Equal(t, expectedHeader.DAH.Hash(), header.DAH.Hash())
}
