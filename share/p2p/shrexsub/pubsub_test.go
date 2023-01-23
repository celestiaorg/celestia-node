package shrexsub

import (
	"context"
	"testing"
	"time"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	mocknet "github.com/libp2p/go-libp2p/p2p/net/mock"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/share"
)

func TestPubSub(t *testing.T) {
	h, err := mocknet.FullMeshConnected(2)
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	t.Cleanup(cancel)

	pSub1, err := NewPubSub(ctx, h.Hosts()[0], "test")
	require.NoError(t, err)

	pSub2, err := NewPubSub(ctx, h.Hosts()[1], "test")
	require.NoError(t, err)

	require.NoError(t, pSub1.Start(ctx))
	require.NoError(t, pSub2.Start(ctx))

	subs, err := pSub2.Subscribe()
	require.NoError(t, err)

	var edsHash share.DataHash = []byte("data")
	err = pSub1.topic.Publish(ctx, edsHash, pubsub.WithReadiness(pubsub.MinTopicSize(1)))
	require.NoError(t, err)

	data, err := subs.Next(ctx)
	require.NoError(t, err)
	require.Equal(t, data, edsHash)
}
