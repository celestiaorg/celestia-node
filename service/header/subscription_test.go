package header

import (
	"context"
	"testing"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	mocknet "github.com/libp2p/go-libp2p/p2p/net/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSubscriber tests the header Service's implementation of Subscriber.
func TestSubscriber(t *testing.T) {
	// create mock network
	net, err := mocknet.FullMeshConnected(context.Background(), 2)
	require.NoError(t, err)

	// get mock host and create new gossipsub on it
	peer1 := net.Hosts()[0]
	gossub1, err := pubsub.NewGossipSub(context.Background(), peer1,
		pubsub.WithMessageSignaturePolicy(pubsub.StrictNoSign))
	require.NoError(t, err)

	// create header Service
	headerServ := NewHeaderService(nil, nil, gossub1)
	err = headerServ.Start(context.Background())
	require.NoError(t, err)

	// subscribe
	subscription, err := headerServ.Subscribe()
	require.NoError(t, err)

	// publish ExtendedHeader to topic
	peer2 := net.Hosts()[1]
	gossub2, err := pubsub.NewGossipSub(context.Background(), peer2,
		pubsub.WithMessageSignaturePolicy(pubsub.StrictNoSign))
	require.NoError(t, err)
	topic, err := gossub2.Join(ExtendedHeaderSubTopic)
	require.NoError(t, err)

	expectedHeader := RandExtendedHeader(t)
	bin, err := expectedHeader.MarshalBinary()
	require.NoError(t, err)

	err = topic.Publish(context.Background(), bin)
	require.NoError(t, err)

	// get next ExtendedHeader from network
	header, err := subscription.NextHeader(context.Background())
	require.NoError(t, err)

	assert.Equal(t, expectedHeader.Height, header.Height)
	assert.Equal(t, expectedHeader.Hash(), header.Hash())
	assert.Equal(t, expectedHeader.DAH.Hash(), header.DAH.Hash())
}
