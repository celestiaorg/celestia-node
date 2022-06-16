package fraud

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	mocknet "github.com/libp2p/go-libp2p/p2p/net/mock"

	"github.com/celestiaorg/celestia-node/ipld"
)

func TestService_RegisterUnmarshaler(t *testing.T) {
	s := createService(t)
	require.NoError(t, s.RegisterUnmarshaler(BadEncoding, UnmarshalBEFP))

	require.Error(t, s.RegisterUnmarshaler(BadEncoding, UnmarshalBEFP))
}

func TestService_UnregisterUnmarshaler(t *testing.T) {
	s := createService(t)
	require.NoError(t, s.RegisterUnmarshaler(BadEncoding, UnmarshalBEFP))
	require.NoError(t, s.UnregisterUnmarshaler(BadEncoding))

	require.Error(t, s.UnregisterUnmarshaler(BadEncoding))
}

func TestService_SubscribeFails(t *testing.T) {
	s := createService(t)

	_, err := s.Subscribe(BadEncoding)
	require.Error(t, err)
}

func TestService_Subscribe(t *testing.T) {
	s := createService(t)
	require.NoError(t, s.RegisterUnmarshaler(BadEncoding, UnmarshalBEFP))

	_, err := s.Subscribe(BadEncoding)
	require.NoError(t, err)
}

func TestService_BroadcastFails(t *testing.T) {
	s := createService(t)
	p := CreateBadEncodingProof([]byte("hash"), 0, &ipld.ErrByzantine{
		Index:  0,
		Shares: make([]*ipld.ShareWithProof, 0),
	},
	)
	require.Error(t, s.Broadcast(context.TODO(), p))
}

func createService(t *testing.T) Service {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*15)
	t.Cleanup(cancel)

	// create mock network
	net, err := mocknet.FullMeshLinked(ctx, 2)
	require.NoError(t, err)

	// create pubsub for host
	ps, err := pubsub.NewGossipSub(ctx, net.Hosts()[0],
		pubsub.WithMessageSignaturePolicy(pubsub.StrictNoSign))
	require.NoError(t, err)

	return NewService(ps, nil)
}
