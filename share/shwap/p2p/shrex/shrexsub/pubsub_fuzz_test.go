package shrexsub

import (
	"bytes"
	"context"
	"testing"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	pubsubpb "github.com/libp2p/go-libp2p-pubsub/pb"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/share"
	pb "github.com/celestiaorg/celestia-node/share/shwap/p2p/shrex/shrexsub/pb"
)

// FuzzNotificationValidate runs the shrex-sub topic validator on an arbitrary
// gossiped message. The validator recovers from panics and rejects the
// message, which would hide a crash, so the decoding and checks it relies on
// also run here outside that recovery. Only a notification with a non-zero
// height and a valid, non-empty data hash may reach the injected ValidatorFn,
// and it must carry exactly what the message encodes.
func FuzzNotificationValidate(f *testing.F) {
	valid := pb.RecentEDSNotification{Height: 42, DataHash: bytes.Repeat([]byte{0xab}, 32)}
	empty := pb.RecentEDSNotification{Height: 42, DataHash: share.EmptyEDSDataHash()}
	zeroHeight := pb.RecentEDSNotification{Height: 0, DataHash: bytes.Repeat([]byte{0xab}, 32)}
	shortHash := pb.RecentEDSNotification{Height: 42, DataHash: []byte{1, 2, 3}}
	for _, msg := range []pb.RecentEDSNotification{valid, empty, zeroHeight, shortHash} {
		raw, err := msg.Marshal()
		require.NoError(f, err)
		f.Add(raw)
	}
	f.Add([]byte{})
	f.Add([]byte{0x0a, 0xff, 0xff, 0xff, 0xff, 0x0f})

	f.Fuzz(func(t *testing.T, raw []byte) {
		var decoded pb.RecentEDSNotification
		decodeErr := decoded.Unmarshal(raw)
		if decodeErr == nil {
			dh := share.DataHash(decoded.DataHash)
			_ = dh.IsEmptyEDS()
			_ = dh.Validate()
			_ = dh.String()
		}

		var got *Notification
		inner := ValidatorFn(func(_ context.Context, _ peer.ID, n Notification) pubsub.ValidationResult {
			got = &n
			return pubsub.ValidationAccept
		})
		msg := &pubsub.Message{Message: &pubsubpb.Message{Data: raw}}
		res := inner.validate(context.Background(), peer.ID(""), msg)

		if got == nil {
			require.Equal(t, pubsub.ValidationReject, res, "a message the validator never saw must be rejected")
			return
		}
		require.Equal(t, pubsub.ValidationAccept, res)
		require.NoError(t, decodeErr, "an undecodable message reached the validator")
		require.NotZero(t, got.Height, "a notification for height 0 reached the validator")
		require.NoError(t, got.DataHash.Validate(), "an invalid data hash reached the validator")
		require.False(t, got.DataHash.IsEmptyEDS(), "an empty-EDS notification reached the validator")
		require.Equal(t, decoded.Height, got.Height)
		require.Equal(t, decoded.DataHash, []byte(got.DataHash))
	})
}
