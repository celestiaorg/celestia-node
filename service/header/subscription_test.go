package header

import (
	"context"
	"testing"
	"time"

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
	ps, err := pubsub.NewGossipSub(context.Background(), net.Hosts()[0],
		pubsub.WithMessageSignaturePolicy(pubsub.StrictNoSign))
	require.NoError(t, err)

	ex, mockStore := createExchangeWithMockStore(t, net.Hosts()[0], net.Hosts()[1])

	// create header Service
	headerServ := NewHeaderService(ex, mockStore, ps, mockStore.head.Hash())
	err = headerServ.Start(context.Background())
	require.NoError(t, err)

	topic := headerServ.topic

	// subscribe
	subscription, err := headerServ.Subscribe()
	require.NoError(t, err)

	time.Sleep(2 * time.Second)

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
