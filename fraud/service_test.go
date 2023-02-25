package fraud

import (
	"context"
	"encoding/hex"
	"testing"
	"time"

	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/namespace"
	"github.com/ipfs/go-datastore/sync"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	pubsubpb "github.com/libp2p/go-libp2p-pubsub/pb"
	"github.com/libp2p/go-libp2p/core/host"
	mocknet "github.com/libp2p/go-libp2p/p2p/net/mock"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/libs/header"
	"github.com/celestiaorg/celestia-node/libs/header/test"
)

func TestService_Subscribe(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*1)
	t.Cleanup(cancel)
	s, _ := CreateTestService(t, false)
	proof := newValidProof()
	require.NoError(t, s.Start(ctx))
	_, err := s.Subscribe(proof.Type())
	require.NoError(t, err)
}

func TestService_SubscribeFails(t *testing.T) {
	s, _ := CreateTestService(t, false)
	proof := newValidProof()
	_, err := s.Subscribe(proof.Type())
	require.Error(t, err)
}

func TestService_BroadcastFails(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*1)
	t.Cleanup(cancel)
	s, _ := CreateTestService(t, false)
	p := newValidProof()
	require.Error(t, s.Broadcast(ctx, p))
}

func TestService_Broadcast(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*15)
	t.Cleanup(cancel)

	s, _ := CreateTestService(t, false)
	proof := newValidProof()
	require.NoError(t, s.Start(ctx))
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
		service, _ := createTestServiceWithHost(ctx, t, net.Hosts()[0], false)
		msg := &pubsub.Message{
			Message: &pubsubpb.Message{
				Data: bin,
			},
			ReceivedFrom: net.Hosts()[1].ID(),
		}
		if test.precondition != nil {
			test.precondition()
		}
		require.NoError(t, service.Start(ctx))
		res := service.processIncoming(ctx, test.proof.Type(), net.Hosts()[1].ID(), msg)
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
	pserviceA, _ := createTestServiceWithHost(ctx, t, net.Hosts()[0], false)
	require.NoError(t, err)
	// create pub sub in order to listen for Fraud Proof
	psB, err := pubsub.NewGossipSub(ctx, net.Hosts()[1], // -> B
		pubsub.WithMessageSignaturePolicy(pubsub.StrictNoSign))
	require.NoError(t, err)
	// create second service that will receive and validate Fraud Proof
	pserviceB := NewProofService(
		psB,
		net.Hosts()[1],
		func(ctx context.Context, u uint64) (header.Header, error) {
			return &test.DummyHeader{}, nil
		},
		sync.MutexWrap(datastore.NewMapDatastore()),
		false,
		"private",
	)
	addrB := host.InfoFromHost(net.Hosts()[1]) // -> B

	// create pub sub in order to listen for Fraud Proof
	psC, err := pubsub.NewGossipSub(ctx, net.Hosts()[2], // -> C
		pubsub.WithMessageSignaturePolicy(pubsub.StrictNoSign))
	require.NoError(t, err)
	pserviceC := NewProofService(
		psC,
		net.Hosts()[2],
		func(ctx context.Context, u uint64) (header.Header, error) {
			return &test.DummyHeader{}, nil
		},
		sync.MutexWrap(datastore.NewMapDatastore()),
		false,
		"private",
	)
	// establish connections
	// connect peers: A -> B -> C, so A and C are not connected to each other
	require.NoError(t, net.Hosts()[0].Connect(ctx, *addrB)) // host[0] is A
	require.NoError(t, net.Hosts()[2].Connect(ctx, *addrB)) // host[2] is C

	befp := newValidProof()
	bin, err := befp.MarshalBinary()
	require.NoError(t, err)
	require.NoError(t, pserviceA.Start(ctx))
	require.NoError(t, pserviceB.Start(ctx))
	require.NoError(t, pserviceC.Start(ctx))
	subsA, err := pserviceA.Subscribe(mockProofType)
	require.NoError(t, err)
	defer subsA.Cancel()

	subsB, err := pserviceB.Subscribe(mockProofType)
	require.NoError(t, err)
	defer subsB.Cancel()

	subsC, err := pserviceC.Subscribe(mockProofType)
	require.NoError(t, err)
	defer subsC.Cancel()
	// we cannot avoid sleep because it helps to avoid flakiness
	time.Sleep(time.Millisecond * 100)

	err = pserviceA.topics[mockProofType].Publish(ctx, bin, pubsub.WithReadiness(pubsub.MinTopicSize(1)))
	require.NoError(t, err)

	newCtx, cancel := context.WithTimeout(ctx, time.Millisecond*100)
	t.Cleanup(cancel)

	_, err = subsB.Proof(newCtx)
	require.NoError(t, err)

	_, err = subsC.Proof(ctx)
	require.NoError(t, err)
	// we cannot avoid sleep because it helps to avoid flakiness
	time.Sleep(time.Millisecond * 100)
}

func TestService_Get(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	t.Cleanup(cancel)
	proof := newValidProof()
	bin, err := proof.MarshalBinary()
	require.NoError(t, err)
	pService, _ := CreateTestService(t, false)
	// try to fetch proof
	_, err = pService.Get(ctx, proof.Type())
	// error is expected here because storage is empty
	require.Error(t, err)

	// create store
	store := initStore(proof.Type(), pService.ds)
	// add proof to storage
	require.NoError(t, put(ctx, store, hex.EncodeToString(proof.HeaderHash()), bin))
	// fetch proof
	_, err = pService.Get(ctx, proof.Type())
	require.NoError(t, err)
}

func TestService_Sync(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	t.Cleanup(cancel)
	// create mock network
	net, err := mocknet.FullMeshLinked(2)
	require.NoError(t, err)

	pserviceA, _ := createTestServiceWithHost(ctx, t, net.Hosts()[0], false)
	pserviceB, _ := createTestServiceWithHost(ctx, t, net.Hosts()[1], true)
	proof := newValidProof()
	require.NoError(t, pserviceA.Start(ctx))
	require.NoError(t, pserviceB.Start(ctx))
	subs, err := pserviceB.Subscribe(mockProofType)
	require.NoError(t, err)
	bin, err := proof.MarshalBinary()
	require.NoError(t, err)
	store := namespace.Wrap(pserviceA.ds, makeKey(mockProofType))
	require.NoError(t, put(ctx, store, string(proof.HeaderHash()), bin))

	addrB := host.InfoFromHost(net.Hosts()[1])
	require.NoError(t, net.Hosts()[0].Connect(ctx, *addrB))

	_, err = subs.Proof(ctx)
	require.NoError(t, err)
}
