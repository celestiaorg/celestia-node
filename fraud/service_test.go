package fraud

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	mdutils "github.com/ipfs/go-merkledag/test"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	mocknet "github.com/libp2p/go-libp2p/p2p/net/mock"

	"github.com/celestiaorg/celestia-node/ipld"
)

func TestService_RegisterUnmarshaler(t *testing.T) {
	s, _ := createService(t)
	require.NoError(t, s.RegisterUnmarshaler(BadEncoding, UnmarshalBEFP))

	require.Error(t, s.RegisterUnmarshaler(BadEncoding, UnmarshalBEFP))
}

func TestService_UnregisterUnmarshaler(t *testing.T) {
	s, _ := createService(t)
	require.NoError(t, s.RegisterUnmarshaler(BadEncoding, UnmarshalBEFP))
	require.NoError(t, s.UnregisterUnmarshaler(BadEncoding))

	require.Error(t, s.UnregisterUnmarshaler(BadEncoding))
}

func TestService_SubscribeFails(t *testing.T) {
	s, _ := createService(t)

	_, err := s.Subscribe(BadEncoding)
	require.Error(t, err)
}

func TestService_Subscribe(t *testing.T) {
	s, _ := createService(t)
	require.NoError(t, s.RegisterUnmarshaler(BadEncoding, UnmarshalBEFP))

	_, err := s.Subscribe(BadEncoding)
	require.NoError(t, err)
}

func TestService_BroadcastFails(t *testing.T) {
	s, _ := createService(t)
	p := CreateBadEncodingProof([]byte("hash"), 0, &ipld.ErrByzantine{
		Index:  0,
		Shares: make([]*ipld.ShareWithProof, 0),
	},
	)
	require.Error(t, s.Broadcast(context.TODO(), p))
}

func TestService_Broadcast(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*15)
	defer t.Cleanup(cancel)

	bServ := mdutils.Bserv()
	s, store := createService(t)
	require.NoError(t, s.RegisterUnmarshaler(BadEncoding, UnmarshalBEFP))
	h, err := store.GetByHeight(context.TODO(), 1)
	require.NoError(t, err)

	faultHeader, err := generateByzantineError(ctx, t, h, bServ)
	require.Error(t, err)
	var errByz *ipld.ErrByzantine
	require.True(t, errors.As(err, &errByz))

	subs, err := s.Subscribe(BadEncoding)
	require.NoError(t, err)
	require.NoError(t, s.Broadcast(ctx, CreateBadEncodingProof([]byte("hash"), uint64(h.Height), errByz)))
	p, err := subs.Proof(ctx)
	require.NoError(t, err)
	require.NoError(t, p.Validate(faultHeader))
}

func createService(t *testing.T) (Service, *mockStore) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*15)
	t.Cleanup(cancel)

	// create mock network
	net, err := mocknet.FullMeshLinked(2)
	require.NoError(t, err)

	// create pubsub for host
	ps, err := pubsub.NewGossipSub(ctx, net.Hosts()[0],
		pubsub.WithMessageSignaturePolicy(pubsub.StrictNoSign))
	require.NoError(t, err)
	store := createStore(t, 10)
	return NewService(ps, store.GetByHeight), store
}
