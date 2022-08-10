package fraud

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/sync"
	mdutils "github.com/ipfs/go-merkledag/test"
	"github.com/libp2p/go-libp2p-core/event"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	mocknet "github.com/libp2p/go-libp2p/p2p/net/mock"

	"github.com/celestiaorg/celestia-node/ipld"
)

func TestService_Subscribe(t *testing.T) {
	s, _ := createService(t)

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
	t.Cleanup(cancel)

	s, store := createService(t)
	h, err := store.GetByHeight(ctx, 1)
	require.NoError(t, err)

	faultHeader, err := generateByzantineError(ctx, t, h, mdutils.Bserv())
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

func TestService_BlackListPeer(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	t.Cleanup(cancel)
	// create mock network
	net, err := mocknet.FullMeshLinked(3)
	require.NoError(t, err)

	// create first fraud service that will broadcast incorrect Fraud Proof
	serviceA, store1 := createServiceWithHost(ctx, t, net.Hosts()[0])

	h, err := store1.GetByHeight(ctx, 1)
	require.NoError(t, err)

	// create and break byzantine error
	_, err = generateByzantineError(ctx, t, h, mdutils.Bserv())
	require.Error(t, err)
	var errByz *ipld.ErrByzantine
	require.True(t, errors.As(err, &errByz))
	errByz.Index = 2

	fserviceA := serviceA.(*service)
	require.NotNil(t, fserviceA)

	// create second service that will receive and validate Fraud Proof
	serviceB, _ := createServiceWithHost(ctx, t, net.Hosts()[1])

	fserviceB := serviceB.(*service)
	require.NotNil(t, fserviceB)

	blackList, err := pubsub.NewTimeCachedBlacklist(time.Hour * 1)
	require.NoError(t, err)
	// create pub sub in order to listen for Fraud Proof
	psC, err := pubsub.NewGossipSub(ctx, net.Hosts()[2], // -> C
		pubsub.WithMessageSignaturePolicy(pubsub.StrictNoSign), pubsub.WithBlacklist(blackList))
	require.NoError(t, err)

	addrB := host.InfoFromHost(net.Hosts()[1]) // -> B

	serviceC := NewService(psC, store1.GetByHeight, sync.MutexWrap(datastore.NewMapDatastore()))

	sub0, err := net.Hosts()[0].EventBus().Subscribe(&event.EvtPeerIdentificationCompleted{})
	require.NoError(t, err)
	sub2, err := net.Hosts()[2].EventBus().Subscribe(&event.EvtPeerIdentificationCompleted{})
	require.NoError(t, err)

	// connect peers: A -> B -> C, so A and C are not connected to each other
	require.NoError(t, net.Hosts()[0].Connect(ctx, *addrB)) // host[0] is A
	require.NoError(t, net.Hosts()[2].Connect(ctx, *addrB)) // host[2] is C

	// wait on both peer identification events
	for i := 0; i < 2; i++ {
		select {
		case <-sub0.Out():
		case <-sub2.Out():
		case <-ctx.Done():
			assert.FailNow(t, "timeout waiting for peers to connect")
		}
	}

	// subscribe to BEFP
	subsA, err := serviceA.Subscribe(BadEncoding)
	require.NoError(t, err)
	defer subsA.Cancel()

	subsB, err := serviceB.Subscribe(BadEncoding)
	require.NoError(t, err)
	defer subsB.Cancel()

	subsC, err := serviceC.Subscribe(BadEncoding)
	require.NoError(t, err)
	defer subsC.Cancel()

	befp := CreateBadEncodingProof([]byte("hash"), uint64(h.Height), errByz)
	// deregister validator in order to send Fraud Proof
	fserviceA.pubsub.UnregisterTopicValidator(getSubTopic(BadEncoding)) //nolint:errcheck
	// create a new validator for serviceB
	fserviceB.pubsub.UnregisterTopicValidator(getSubTopic(BadEncoding)) //nolint:errcheck
	f := func(ctx context.Context, from peer.ID, msg *pubsub.Message) pubsub.ValidationResult {
		msg.ValidatorData = befp
		return pubsub.ValidationAccept
	}
	fserviceB.pubsub.RegisterTopicValidator(getSubTopic(BadEncoding), f) //nolint:errcheck
	bin, err := befp.MarshalBinary()
	require.NoError(t, err)
	topic, ok := fserviceA.topics[BadEncoding]
	require.True(t, ok)
	// we cannot avoid sleep because it helps to avoid flakiness
	time.Sleep(time.Millisecond * 100)
	err = topic.Publish(ctx, bin, pubsub.WithReadiness(pubsub.MinTopicSize(1)))
	require.NoError(t, err)

	_, err = subsB.Proof(ctx)
	require.NoError(t, err)

	newCtx, cancel := context.WithTimeout(ctx, time.Second*1)
	t.Cleanup(cancel)

	_, err = subsC.Proof(newCtx)
	require.Error(t, err)
	require.False(t, blackList.Contains(net.Hosts()[0].ID()))
	require.True(t, blackList.Contains(net.Hosts()[1].ID()))
	// we cannot avoid sleep because it helps to avoid flakiness
	time.Sleep(time.Millisecond * 100)
}

func TestService_GossipingOfFaultBEFP(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	t.Cleanup(cancel)
	// create mock network
	net, err := mocknet.FullMeshLinked(3)
	require.NoError(t, err)

	// create first fraud service that will broadcast incorrect Fraud Proof
	serviceA, store1 := createServiceWithHost(ctx, t, net.Hosts()[0])

	h, err := store1.GetByHeight(ctx, 1)
	require.NoError(t, err)

	// create and break byzantine error
	_, err = generateByzantineError(ctx, t, h, mdutils.Bserv())
	require.Error(t, err)
	var errByz *ipld.ErrByzantine
	require.True(t, errors.As(err, &errByz))
	errByz.Index = 2

	fserviceA := serviceA.(*service)
	require.NotNil(t, fserviceA)

	blackList, err := pubsub.NewTimeCachedBlacklist(time.Hour)
	require.NoError(t, err)
	// create pub sub in order to listen for Fraud Proof
	psB, err := pubsub.NewGossipSub(ctx, net.Hosts()[1], // -> B
		pubsub.WithMessageSignaturePolicy(pubsub.StrictNoSign), pubsub.WithBlacklist(blackList))
	require.NoError(t, err)
	// create second service that will receive and validate Fraud Proof
	serviceB := NewService(psB, store1.GetByHeight, sync.MutexWrap(datastore.NewMapDatastore()))
	fserviceB := serviceB.(*service)
	require.NotNil(t, fserviceB)
	addrB := host.InfoFromHost(net.Hosts()[1]) // -> B

	// create pub sub in order to listen for Fraud Proof
	psC, err := pubsub.NewGossipSub(ctx, net.Hosts()[2], // -> C
		pubsub.WithMessageSignaturePolicy(pubsub.StrictNoSign))
	require.NoError(t, err)
	serviceC := NewService(psC, store1.GetByHeight, sync.MutexWrap(datastore.NewMapDatastore()))

	// perform subscriptions
	sub0, err := net.Hosts()[0].EventBus().Subscribe(&event.EvtPeerIdentificationCompleted{})
	require.NoError(t, err)
	sub2, err := net.Hosts()[2].EventBus().Subscribe(&event.EvtPeerIdentificationCompleted{})
	require.NoError(t, err)

	// establish connections
	// connect peers: A -> B -> C, so A and C are not connected to each other
	require.NoError(t, net.Hosts()[0].Connect(ctx, *addrB)) // host[0] is A
	require.NoError(t, net.Hosts()[2].Connect(ctx, *addrB)) // host[2] is C

	// wait on both peer identification events
	for i := 0; i < 2; i++ {
		select {
		case <-sub0.Out():
		case <-sub2.Out():
		case <-ctx.Done():
			assert.FailNow(t, "timeout waiting for peers to connect")
		}
	}

	// subscribe to BEFP
	subsA, err := serviceA.Subscribe(BadEncoding)
	require.NoError(t, err)
	defer subsA.Cancel()

	subsB, err := serviceB.Subscribe(BadEncoding)
	require.NoError(t, err)
	defer subsB.Cancel()

	subsC, err := serviceC.Subscribe(BadEncoding)
	require.NoError(t, err)
	defer subsC.Cancel()

	// deregister validator in order to send Fraud Proof
	fserviceA.pubsub.UnregisterTopicValidator(getSubTopic(BadEncoding)) //nolint:errcheck
	// Broadcast BEFP
	befp := CreateBadEncodingProof([]byte("hash"), uint64(h.Height), errByz)
	bin, err := befp.MarshalBinary()
	require.NoError(t, err)
	// we cannot avoid sleep because it helps to avoid flakiness
	time.Sleep(time.Millisecond * 100)
	err = fserviceA.topics[BadEncoding].Publish(ctx, bin, pubsub.WithReadiness(pubsub.MinTopicSize(1)))
	require.NoError(t, err)

	newCtx, cancel := context.WithTimeout(ctx, time.Millisecond*100)
	t.Cleanup(cancel)

	_, err = subsB.Proof(newCtx)
	require.Error(t, err)
	require.True(t, blackList.Contains(net.Hosts()[0].ID()))

	proofs, err := serviceC.Get(ctx, BadEncoding)
	require.Error(t, err)
	require.Nil(t, proofs)
	// we cannot avoid sleep because it helps to avoid flakiness
	time.Sleep(time.Millisecond * 100)
}

func TestService_GossipingOfBEFP(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	t.Cleanup(cancel)
	// create mock network
	net, err := mocknet.FullMeshLinked(3)
	require.NoError(t, err)

	// create first fraud service that will broadcast incorrect Fraud Proof
	serviceA, store1 := createServiceWithHost(ctx, t, net.Hosts()[0])

	h, err := store1.GetByHeight(ctx, 1)
	require.NoError(t, err)

	// create and break byzantine error
	_, err = generateByzantineError(ctx, t, h, mdutils.Bserv())
	require.Error(t, err)
	var errByz *ipld.ErrByzantine
	require.True(t, errors.As(err, &errByz))

	fserviceA := serviceA.(*service)
	require.NotNil(t, fserviceA)

	// create pub sub in order to listen for Fraud Proof
	psB, err := pubsub.NewGossipSub(ctx, net.Hosts()[1], // -> B
		pubsub.WithMessageSignaturePolicy(pubsub.StrictNoSign))
	require.NoError(t, err)
	// create second service that will receive and validate Fraud Proof
	serviceB := NewService(psB, store1.GetByHeight, sync.MutexWrap(datastore.NewMapDatastore()))
	fserviceB := serviceB.(*service)
	require.NotNil(t, fserviceB)
	addrB := host.InfoFromHost(net.Hosts()[1]) // -> B

	// create pub sub in order to listen for Fraud Proof
	psC, err := pubsub.NewGossipSub(ctx, net.Hosts()[2], // -> C
		pubsub.WithMessageSignaturePolicy(pubsub.StrictNoSign))
	require.NoError(t, err)
	serviceC := NewService(psC, store1.GetByHeight, sync.MutexWrap(datastore.NewMapDatastore()))

	// perform subscriptions
	sub0, err := net.Hosts()[0].EventBus().Subscribe(&event.EvtPeerIdentificationCompleted{})
	require.NoError(t, err)
	sub2, err := net.Hosts()[2].EventBus().Subscribe(&event.EvtPeerIdentificationCompleted{})
	require.NoError(t, err)

	// establish connections
	// connect peers: A -> B -> C, so A and C are not connected to each other
	require.NoError(t, net.Hosts()[0].Connect(ctx, *addrB)) // host[0] is A
	require.NoError(t, net.Hosts()[2].Connect(ctx, *addrB)) // host[2] is C

	// wait on both peer identification events
	for i := 0; i < 2; i++ {
		select {
		case <-sub0.Out():
		case <-sub2.Out():
		case <-ctx.Done():
			assert.FailNow(t, "timeout waiting for peers to connect")
		}
	}

	// subscribe to BEFP
	subsA, err := serviceA.Subscribe(BadEncoding)
	require.NoError(t, err)
	defer subsA.Cancel()

	subsB, err := serviceB.Subscribe(BadEncoding)
	require.NoError(t, err)
	defer subsB.Cancel()

	subsC, err := serviceC.Subscribe(BadEncoding)
	require.NoError(t, err)
	defer subsC.Cancel()

	// deregister validator in order to send Fraud Proof
	fserviceA.pubsub.UnregisterTopicValidator(getSubTopic(BadEncoding)) //nolint:errcheck
	// Broadcast BEFP
	befp := CreateBadEncodingProof([]byte("hash"), uint64(h.Height), errByz)
	bin, err := befp.MarshalBinary()
	require.NoError(t, err)
	// we cannot avoid sleep because it helps to avoid flakiness
	time.Sleep(time.Millisecond * 100)
	err = fserviceA.topics[BadEncoding].Publish(ctx, bin, pubsub.WithReadiness(pubsub.MinTopicSize(1)))
	require.NoError(t, err)

	newCtx, cancel := context.WithTimeout(ctx, time.Millisecond*100)
	t.Cleanup(cancel)

	p, err := subsB.Proof(newCtx)
	require.NoError(t, err)
	require.NoError(t, p.Validate(h))

	p, err = subsC.Proof(ctx)
	require.NoError(t, err)
	require.NoError(t, p.Validate(h))

	proofs, err := serviceC.Get(ctx, BadEncoding)
	require.NoError(t, err)
	require.NoError(t, proofs[0].Validate(h))
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
