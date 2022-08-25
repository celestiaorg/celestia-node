package fraud

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/sync"
	"github.com/libp2p/go-libp2p-core/host"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	pubsubpb "github.com/libp2p/go-libp2p-pubsub/pb"
	mocknet "github.com/libp2p/go-libp2p/p2p/net/mock"

	"github.com/celestiaorg/celestia-node/header"
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
				delete(supportedProofTypes, ProofType(-1))
			},
			newValidProof(),
			pubsub.ValidationReject,
		},
		{
			func() {
				delete(defaultUnmarshalers, ProofType(-1))
			},
			newInvalidProof(),
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

func TestService_ReGossiping(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	t.Cleanup(cancel)
	// create mock network
	net, err := mocknet.FullMeshLinked(3)
	require.NoError(t, err)

	// create first fraud service that will broadcast incorrect Fraud Proof
	serviceA, _ := createServiceWithHost(ctx, t, net.Hosts()[0])
	serviceB, _ := createServiceWithHost(ctx, t, net.Hosts()[1])
	serviceC, _ := createServiceWithHost(ctx, t, net.Hosts()[2])

	fserviceA := serviceA.(*service)
	require.NotNil(t, fserviceA)
	fserviceB := serviceB.(*service)
	require.NotNil(t, fserviceB)
	fserviceC := serviceC.(*service)
	require.NotNil(t, fserviceC)
	addrB := host.InfoFromHost(net.Hosts()[1]) // -> B

	// establish connections
	// connect peers: A -> B -> C, so A and C are not connected to each other
	require.NoError(t, net.Hosts()[0].Connect(ctx, *addrB)) // host[0] is A
	require.NoError(t, net.Hosts()[2].Connect(ctx, *addrB)) // host[2] is C
	proof := newValidProof()
	// subscribe to fraud proof
	subsA, err := serviceA.Subscribe(proof.Type())
	require.NoError(t, err)
	defer subsA.Cancel()

	subsB, err := serviceB.Subscribe(proof.Type())
	require.NoError(t, err)
	defer subsB.Cancel()

	subsC, err := serviceC.Subscribe(proof.Type())
	require.NoError(t, err)
	defer subsC.Cancel()

	bin, err := proof.MarshalBinary()
	require.NoError(t, err)
	err = fserviceA.topics[proof.Type()].Publish(ctx, bin, pubsub.WithReadiness(pubsub.MinTopicSize(1)))
	require.NoError(t, err)

	newCtx, cancel := context.WithTimeout(ctx, time.Millisecond*100)
	t.Cleanup(cancel)

	_, err = subsB.Proof(newCtx)
	require.NoError(t, err)
	require.NoError(t, nil)

	_, err = subsC.Proof(ctx)
	require.NoError(t, err)
	require.NoError(t, nil)

	proofs, err := serviceC.Get(ctx, proof.Type())
	require.NoError(t, err)
	require.NoError(t, proofs[0].Validate(&header.ExtendedHeader{}))
	// we cannot avoid sleep because it helps to avoid flakiness
	time.Sleep(time.Millisecond * 100)
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
