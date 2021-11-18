package header

import (
	"context"
	"testing"
	"time"

	"github.com/ipfs/go-datastore"
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
	net, err := mocknet.FullMeshConnected(ctx, 2)
	require.NoError(t, err)

	suite := NewTestSuite(t, 3)

	// get mock host and create new gossipsub on it
	pubsub1, err := pubsub.NewGossipSub(ctx, net.Hosts()[0], pubsub.WithMessageSignaturePolicy(pubsub.StrictNoSign))
	require.NoError(t, err)

	store1, err := NewStoreWithHead(datastore.NewMapDatastore(), suite.Head())
	require.NoError(t, err)

	// create header Service
	headerServ1 := NewHeaderService(NewLocalExchange(store1), store1, pubsub1, suite.Head().Hash())
	err = headerServ1.Start(ctx)
	require.NoError(t, err)

	// subscribe
	subscription, err := headerServ1.Subscribe()
	require.NoError(t, err)

	// get mock host and create new gossipsub on it
	pubsub2, err := pubsub.NewGossipSub(ctx, net.Hosts()[1],
		pubsub.WithMessageSignaturePolicy(pubsub.StrictNoSign))
	require.NoError(t, err)

	store2, err := NewStoreWithHead(datastore.NewMapDatastore(), suite.Head())
	require.NoError(t, err)

	// create header Service
	headerServ2 := NewHeaderService(NewLocalExchange(store2), store2, pubsub2, suite.Head().Hash())
	err = headerServ2.Start(ctx)
	require.NoError(t, err)

	_, err = headerServ2.Subscribe()
	require.NoError(t, err)

	time.Sleep(2 * time.Second)

	expectedHeader := suite.GenExtendedHeaders(1)[0]
	bin, err := expectedHeader.MarshalBinary()
	require.NoError(t, err)

	err = headerServ2.topic.Publish(ctx, bin)
	require.NoError(t, err)

	// get next ExtendedHeader from network
	header, err := subscription.NextHeader(ctx)
	require.NoError(t, err)

	assert.Equal(t, expectedHeader.Height, header.Height)
	assert.Equal(t, expectedHeader.Hash(), header.Hash())
	assert.Equal(t, expectedHeader.DAH.Hash(), header.DAH.Hash())
}
