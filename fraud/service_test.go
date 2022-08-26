package fraud

import (
	"context"
	"encoding/hex"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/sync"
	"github.com/libp2p/go-libp2p-core/host"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	pubsubpb "github.com/libp2p/go-libp2p-pubsub/pb"
	mocknet "github.com/libp2p/go-libp2p/p2p/net/mock"
)

func TestService_Subscribe(t *testing.T) {
	s, _ := createService(t)
	proof := newValidProof()
	_, err := s.Subscribe(proof.Type())
	require.NoError(t, err)
}

func TestService_SubscribeFails(t *testing.T) {
	s, _ := createService(t)
	proof := newValidProof()
	delete(defaultUnmarshalers, proof.Type())
	_, err := s.Subscribe(proof.Type())
	require.NoError(t, err)
}

func TestService_BroadcastFails(t *testing.T) {
	s, _ := createService(t)
	p := newValidProof()
	require.Error(t, s.Broadcast(context.TODO(), p))
}

func TestService_Broadcast(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*15)
	t.Cleanup(cancel)

	s, _ := createService(t)

	proof := newValidProof()
	subs, err := s.Subscribe(proof.Type())
	require.NoError(t, err)

	require.NoError(t, s.Broadcast(ctx, proof))
	_, err = subs.Proof(ctx)
	require.NoError(t, err)
	require.NoError(t, nil)
}

func TestService_processIncoming(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	t.Cleanup(cancel)
	// create mock network
	net, err := mocknet.FullMeshLinked(2)
	require.NoError(t, err)

	var tests = []struct {
		precondition     func()
		proof            *mockProof
		validationResult pubsub.ValidationResult
	}{
		{
			nil,
			newValidProof(),
			pubsub.ValidationAccept,
		},
		{
			nil,
			newInvalidProof(),
			pubsub.ValidationReject,
		},
		{
			func() {
				delete(defaultUnmarshalers, mockProofType)
			},
			newValidProof(),
			pubsub.ValidationReject,
		},
	}
	for _, test := range tests {
		bin, err := test.proof.MarshalBinary()
		require.NoError(t, err)
		// create first fraud service that will broadcast incorrect Fraud Proof
		serviceA, _ := createServiceWithHost(ctx, t, net.Hosts()[0])
		fserviceA := serviceA.(*service)
		require.NotNil(t, fserviceA)
		msg := &pubsub.Message{
			Message: &pubsubpb.Message{
				Data: bin,
			},
			ReceivedFrom: net.Hosts()[1].ID(),
		}
		if test.precondition != nil {
			test.precondition()
		}
		res := fserviceA.processIncoming(ctx, test.proof.Type(), net.Hosts()[1].ID(), msg)
		require.True(t, res == test.validationResult)
	}
}

func TestService_Get(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	t.Cleanup(cancel)
	proof := newValidProof()
	bin, err := proof.MarshalBinary()
	require.NoError(t, err)
	pService, _ := createService(t)
	service := pService.(*service)
	require.NotNil(t, service)

	// try to fetch proof
	_, err = pService.Get(ctx, proof.Type())
	// error is expected here because storage is empty
	require.Error(t, err)

	// create store
	store := initStore(proof.Type(), service.ds)
	// add proof to storage
	require.NoError(t, put(ctx, store, hex.EncodeToString(proof.HeaderHash()), bin))
	// fetch proof
	_, err = pService.Get(ctx, proof.Type())
	require.NoError(t, err)
}

func createService(t *testing.T) (Service, *mockStore) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*15)
	t.Cleanup(cancel)

	// create mock network
	net, err := mocknet.FullMeshLinked(1)
	require.NoError(t, err)
	// create pubsub for host
	ps, err := pubsub.NewGossipSub(ctx, net.Hosts()[0],
		pubsub.WithMessageSignaturePolicy(pubsub.StrictNoSign))
	require.NoError(t, err)
	store := createStore(t, 10)
	return NewService(ps, store.GetByHeight, sync.MutexWrap(datastore.NewMapDatastore())), store
}

func createServiceWithHost(ctx context.Context, t *testing.T, host host.Host) (Service, *mockStore) {
	// create pubsub for host
	ps, err := pubsub.NewGossipSub(ctx, host,
		pubsub.WithMessageSignaturePolicy(pubsub.StrictNoSign))
	require.NoError(t, err)
	store := createStore(t, 10)
	return NewService(ps, store.GetByHeight, sync.MutexWrap(datastore.NewMapDatastore())), store
}
