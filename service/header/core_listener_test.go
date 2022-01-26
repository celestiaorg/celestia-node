package header

import (
	"context"
	"testing"

	mdutils "github.com/ipfs/go-merkledag/test"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/core"
)

// TestCoreListener tests the lifecycle of the core listener.
func TestCoreListener(t *testing.T) {
	// create mocknet with two pubsub endpoints
	ps1, ps2 := createMocknetWithTwoPubsubEndpoints(t)
	// create second subscription endpoint to listen for CoreListener's pubsub messages
	topic2, err := ps2.Join(PubSubTopic)
	require.NoError(t, err)
	sub, err := topic2.Subscribe()
	require.NoError(t, err)

	// create one block to store as Head in local store and then unsubscribe from block events
	fetcher := createCoreFetcher(t)

	// create CoreListener and start listening
	cl := createCoreListener(t, fetcher, ps1)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	err = cl.Start(ctx)
	require.NoError(t, err)

	// ensure headers are getting broadcasted to the gossipsub topic
	for i := 1; i < 6; i++ {
		msg, err := sub.Next(context.Background())
		require.NoError(t, err)

		var resp ExtendedHeader
		err = resp.UnmarshalBinary(msg.Data)
		require.NoError(t, err)
		require.Equal(t, i, int(resp.Height))
	}

	err = cl.Stop(ctx)
	require.NoError(t, err)
	require.Nil(t, cl.cancel)
}

func createMocknetWithTwoPubsubEndpoints(t *testing.T) (*pubsub.PubSub, *pubsub.PubSub) {
	host1, host2 := createMocknet(context.Background(), t)
	// create pubsub for host
	ps1, err := pubsub.NewGossipSub(context.Background(), host1,
		pubsub.WithMessageSignaturePolicy(pubsub.StrictNoSign))
	require.NoError(t, err)
	// create pubsub for peer-side (to test broadcast comes through network)
	ps2, err := pubsub.NewGossipSub(context.Background(), host2,
		pubsub.WithMessageSignaturePolicy(pubsub.StrictNoSign))
	require.NoError(t, err)
	return ps1, ps2
}

func createCoreListener(
	t *testing.T,
	fetcher *core.BlockFetcher,
	ps *pubsub.PubSub,
) *CoreListener {
	p2pSub := NewP2PSubscriber(ps)
	err := p2pSub.Start(context.Background())
	require.NoError(t, err)
	t.Cleanup(func() {
		p2pSub.Stop(context.Background()) //nolint:errcheck
	})

	return NewCoreListener(p2pSub, fetcher, mdutils.Mock())
}
