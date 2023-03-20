package shrexsub

import (
	"context"
	"testing"
	"time"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	mocknet "github.com/libp2p/go-libp2p/p2p/net/mock"
	"github.com/stretchr/testify/require"

	pb "github.com/celestiaorg/celestia-node/share/p2p/shrexsub/pb"
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

	notification := Notification{
		DataHash: []byte("data"),
		Height:   1,
	}

	msg := pb.EDSAvailableMessage{
		Height:   notification.Height,
		DataHash: notification.DataHash,
	}

	data, err := msg.Marshal()
	require.NoError(t, err)

	err = pSub1.topic.Publish(ctx, data, pubsub.WithReadiness(pubsub.MinTopicSize(1)))
	require.NoError(t, err)

	got, err := subs.Next(ctx)
	require.NoError(t, err)
	require.Equal(t, notification, got)
}
