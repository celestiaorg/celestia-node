package shrexsub

import (
	"context"
	"testing"
	"time"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/peer"
	mocknet "github.com/libp2p/go-libp2p/p2p/net/mock"
	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/libs/rand"

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
	err = pSub2.AddValidator(
		func(ctx context.Context, p peer.ID, n Notification) pubsub.ValidationResult {
			// only testing shrexsub validation here
			return pubsub.ValidationAccept
		},
	)
	require.NoError(t, err)

	require.NoError(t, pSub1.Start(ctx))
	require.NoError(t, pSub2.Start(ctx))

	subs, err := pSub2.Subscribe()
	require.NoError(t, err)

	tests := []struct {
		name        string
		notif       Notification
		errExpected bool
	}{
		{
			name: "valid height, valid hash",
			notif: Notification{
				Height:   1,
				DataHash: rand.Bytes(32),
			},
			errExpected: false,
		},
		{
			name: "valid height, invalid hash (<32 bytes)",
			notif: Notification{
				Height:   2,
				DataHash: rand.Bytes(20),
			},
			errExpected: true,
		},
		{
			name: "valid height, invalid hash (>32 bytes)",
			notif: Notification{
				Height:   2,
				DataHash: rand.Bytes(64),
			},
			errExpected: true,
		},
		{
			name: "invalid height, valid hash",
			notif: Notification{
				Height:   0,
				DataHash: rand.Bytes(32),
			},
			errExpected: true,
		},
		{
			name: "invalid height, nil hash",
			notif: Notification{
				Height:   0,
				DataHash: nil,
			},
			errExpected: true,
		},
		{
			name: "valid height, nil hash",
			notif: Notification{
				Height:   30,
				DataHash: nil,
			},
			errExpected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := pb.RecentEDSNotification{
				Height:   tt.notif.Height,
				DataHash: tt.notif.DataHash,
			}
			data, err := msg.Marshal()
			require.NoError(t, err)

			err = pSub1.topic.Publish(ctx, data, pubsub.WithReadiness(pubsub.MinTopicSize(1)))
			require.NoError(t, err)

			reqCtx, reqCtxCancel := context.WithTimeout(context.Background(), time.Millisecond*100)
			defer reqCtxCancel()

			got, err := subs.Next(reqCtx)
			if tt.errExpected {
				require.Error(t, err)
				require.ErrorIs(t, err, context.DeadlineExceeded)
				return
			}
			require.NoError(t, err)
			require.NoError(t, err)
			require.Equal(t, tt.notif, got)
		})
	}
}
