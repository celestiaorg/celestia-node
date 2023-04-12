package fraudserv

import (
	"context"
	"testing"
	"time"

	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/sync"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/host"
	mocknet "github.com/libp2p/go-libp2p/p2p/net/mock"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/go-header"
	"github.com/celestiaorg/go-header/headertest"

	"github.com/celestiaorg/celestia-node/libs/fraud/fraudtest"
)

func TestService_SubscribeBroadcastValid(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	t.Cleanup(cancel)

	serv := newTestService(ctx, t, false)
	require.NoError(t, serv.Start(ctx))

	fraud := fraudtest.NewValidProof()
	sub, err := serv.Subscribe(fraud.Type())
	require.NoError(t, err)
	defer sub.Cancel()

	require.NoError(t, serv.Broadcast(ctx, fraud))
	_, err = sub.Proof(ctx)
	require.NoError(t, err)
}

func TestService_SubscribeBroadcastInvalid(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	t.Cleanup(cancel)

	serv := newTestService(ctx, t, false)
	require.NoError(t, serv.Start(ctx))

	fraud := fraudtest.NewInvalidProof()
	sub, err := serv.Subscribe(fraud.Type())
	require.NoError(t, err)
	defer sub.Cancel()

	err = serv.Broadcast(ctx, fraud)
	require.Error(t, err)

	ctx2, cancel := context.WithTimeout(context.Background(), time.Millisecond*100)
	t.Cleanup(cancel)

	_, err = sub.Proof(ctx2)
	require.Error(t, err)
}

func TestService_ReGossiping(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	t.Cleanup(cancel)

	// create mock network
	net, err := mocknet.FullMeshLinked(3)
	require.NoError(t, err)

	// create services
	servA := newTestServiceWithHost(ctx, t, net.Hosts()[0], false)
	servB := newTestServiceWithHost(ctx, t, net.Hosts()[1], false)
	servC := newTestServiceWithHost(ctx, t, net.Hosts()[2], false)

	// preconnect peers: A -> B -> C, so A and C are not connected to each other
	addrB := host.InfoFromHost(net.Hosts()[1])              // -> B
	require.NoError(t, net.Hosts()[0].Connect(ctx, *addrB)) // host[0] is A
	require.NoError(t, net.Hosts()[2].Connect(ctx, *addrB)) // host[2] is C

	// start services
	require.NoError(t, servA.Start(ctx))
	require.NoError(t, servB.Start(ctx))
	require.NoError(t, servC.Start(ctx))

	fraud := fraudtest.NewValidProof()
	subsA, err := servA.Subscribe(fraud.Type())
	require.NoError(t, err)
	defer subsA.Cancel()

	subsB, err := servB.Subscribe(fraud.Type())
	require.NoError(t, err)
	defer subsB.Cancel()

	subsC, err := servC.Subscribe(fraud.Type())
	require.NoError(t, err)
	defer subsC.Cancel()

	// give some time for subscriptions to land
	// this mitigates flakiness
	time.Sleep(time.Millisecond * 100)

	// and only after broadcaster
	err = servA.Broadcast(ctx, fraud)
	require.NoError(t, err)

	_, err = subsA.Proof(ctx) // subscriptions of subA should also receive the proof
	require.NoError(t, err)

	_, err = subsB.Proof(ctx)
	require.NoError(t, err)

	_, err = subsC.Proof(ctx)
	require.NoError(t, err)
}

func TestService_Get(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	t.Cleanup(cancel)

	serv := newTestService(ctx, t, false)
	require.NoError(t, serv.Start(ctx))

	fraud := fraudtest.NewValidProof()
	_, err := serv.Get(ctx, fraud.Type()) // try to fetch proof
	require.Error(t, err)                 // storage is empty so should error

	sub, err := serv.Subscribe(fraud.Type())
	require.NoError(t, err)
	defer sub.Cancel()

	// subscription needs some time and love to avoid flakes
	time.Sleep(time.Millisecond * 100)

	err = serv.Broadcast(ctx, fraud) // broadcast stores the fraud as well
	require.NoError(t, err)

	_, err = serv.Get(ctx, fraud.Type())
	require.NoError(t, err)
}

func TestService_Sync(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	t.Cleanup(cancel)

	net, err := mocknet.FullMeshLinked(2)
	require.NoError(t, err)

	servA := newTestServiceWithHost(ctx, t, net.Hosts()[0], false)
	require.NoError(t, servA.Start(ctx))

	fraud := fraudtest.NewValidProof()
	err = servA.Broadcast(ctx, fraud) // broadcasting ensures the fraud gets stored on servA
	require.NoError(t, err)

	servB := newTestServiceWithHost(ctx, t, net.Hosts()[1], true) // start servB
	require.NoError(t, servB.Start(ctx))

	sub, err := servB.Subscribe(fraud.Type()) // subscribe
	require.NoError(t, err)
	defer sub.Cancel()

	addrB := host.InfoFromHost(net.Hosts()[1])
	require.NoError(t, net.Hosts()[0].Connect(ctx, *addrB)) // connect A to B

	_, err = sub.Proof(ctx) // heck that we get it from subscription by syncing from servA
	require.NoError(t, err)
}

func newTestService(ctx context.Context, t *testing.T, enabledSyncer bool) *ProofService {
	net, err := mocknet.FullMeshLinked(1)
	require.NoError(t, err)
	return newTestServiceWithHost(ctx, t, net.Hosts()[0], enabledSyncer)
}

func newTestServiceWithHost(ctx context.Context, t *testing.T, host host.Host, enabledSyncer bool) *ProofService {
	ps, err := pubsub.NewFloodSub(ctx, host, pubsub.WithMessageSignaturePolicy(pubsub.StrictNoSign))
	require.NoError(t, err)

	store := headertest.NewDummyStore(t)
	serv := NewProofService(
		ps,
		host,
		func(ctx context.Context, u uint64) (header.Header, error) {
			return store.GetByHeight(ctx, u)
		},
		sync.MutexWrap(datastore.NewMapDatastore()),
		enabledSyncer,
		"private",
	)

	t.Cleanup(func() {
		err := serv.Stop(ctx)
		if err != nil {
			t.Fatal(err)
		}
	})
	return serv
}
