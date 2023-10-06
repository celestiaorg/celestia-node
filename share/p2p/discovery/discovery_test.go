package discovery

import (
	"context"
	"testing"
	"time"

	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/discovery"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/discovery/routing"
	basic "github.com/libp2p/go-libp2p/p2p/host/basic"
	"github.com/libp2p/go-libp2p/p2p/host/eventbus"
	swarmt "github.com/libp2p/go-libp2p/p2p/net/swarm/testing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDiscovery(t *testing.T) {
	const nodes = 10 // higher number brings higher coverage

	discoveryRetryTimeout = time.Millisecond * 100 // defined in discovery.go

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*30)
	t.Cleanup(cancel)

	tn := newTestnet(ctx, t)

	type peerUpdate struct {
		peerID  peer.ID
		isAdded bool
	}
	updateCh := make(chan peerUpdate)
	submit := func(peerID peer.ID, isAdded bool) {
		updateCh <- peerUpdate{peerID: peerID, isAdded: isAdded}
	}

	host, routingDisc := tn.peer()
	params := DefaultParameters()
	params.PeersLimit = nodes
	params.AdvertiseInterval = -1
	params.Tag = fullNodesTag

	peerA := tn.discovery(params, host, routingDisc,
		WithOnPeersUpdate(submit),
	)

	params = &Parameters{
		PeersLimit:        0,
		AdvertiseInterval: time.Millisecond * 100,
		Tag:               fullNodesTag,
	}
	discs := make([]*Discovery, nodes)
	for i := range discs {
		host, routingDisc := tn.peer()
		discs[i] = tn.discovery(params, host, routingDisc)

		select {
		case res := <-updateCh:
			require.Equal(t, discs[i].host.ID(), res.peerID)
			require.True(t, res.isAdded)
		case <-ctx.Done():
			t.Fatal("did not discover peer in time")
		}
	}

	assert.EqualValues(t, nodes, peerA.set.Size())

	for _, disc := range discs {
		peerID := disc.host.ID()
		err := peerA.host.Network().ClosePeer(peerID)
		require.NoError(t, err)

		select {
		case res := <-updateCh:
			require.Equal(t, peerID, res.peerID)
			require.False(t, res.isAdded)
		case <-ctx.Done():
			t.Fatal("did not disconnect from peer in time")
		}
	}

	assert.EqualValues(t, 0, peerA.set.Size())
}

func TestDiscoveryTagged(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	t.Cleanup(cancel)

	tn := newTestnet(ctx, t)

	// launch 2 peers, that advertise with different tags
	adv1, routingDisc1 := tn.peer()
	adv2, routingDisc2 := tn.peer()

	// sub will discover both peers, but on different tags
	sub, routingDisc := tn.peer()

	params := DefaultParameters()

	// create 2 discovery services for sub, each with a different tag
	params.Tag = "tag1"
	done1 := make(chan struct{})
	tn.discovery(params, sub, routingDisc,
		WithOnPeersUpdate(checkPeer(t, adv1.ID(), done1)))

	params.Tag = "tag2"
	done2 := make(chan struct{})
	tn.discovery(params, sub, routingDisc,
		WithOnPeersUpdate(checkPeer(t, adv2.ID(), done2)))

	// run discovery services for advertisers
	params.Tag = "tag1"
	tn.discovery(params, adv1, routingDisc1)

	params.Tag = "tag2"
	tn.discovery(params, adv2, routingDisc2)

	// wait for discovery services to discover each other on different tags
	select {
	case <-done1:
	case <-ctx.Done():
		t.Fatal("did not discover peer in time")
	}

	select {
	case <-done2:
	case <-ctx.Done():
		t.Fatal("did not discover peer in time")
	}
}

type testnet struct {
	ctx context.Context
	T   *testing.T

	bootstrapper peer.AddrInfo
}

func newTestnet(ctx context.Context, t *testing.T) *testnet {
	bus := eventbus.NewBus()
	swarm := swarmt.GenSwarm(t, swarmt.OptDisableTCP, swarmt.EventBus(bus))
	hst, err := basic.NewHost(swarm, &basic.HostOpts{EventBus: bus})
	require.NoError(t, err)
	hst.Start()

	_, err = dht.New(ctx, hst,
		dht.Mode(dht.ModeServer),
		dht.BootstrapPeers(),
		dht.ProtocolPrefix("/test"),
	)
	require.NoError(t, err)

	return &testnet{ctx: ctx, T: t, bootstrapper: *host.InfoFromHost(hst)}
}

func (t *testnet) discovery(
	params *Parameters,
	hst host.Host,
	routingDisc discovery.Discovery,
	opts ...Option,
) *Discovery {
	disc, err := NewDiscovery(params, hst, routingDisc, opts...)
	require.NoError(t.T, err)
	err = disc.Start(t.ctx)
	require.NoError(t.T, err)
	t.T.Cleanup(func() {
		err := disc.Stop(t.ctx)
		require.NoError(t.T, err)
	})

	go disc.Advertise(t.ctx)
	return disc
}

func (t *testnet) peer() (host.Host, discovery.Discovery) {
	bus := eventbus.NewBus()
	swarm := swarmt.GenSwarm(t.T, swarmt.OptDisableTCP, swarmt.EventBus(bus))
	hst, err := basic.NewHost(swarm, &basic.HostOpts{EventBus: bus})
	require.NoError(t.T, err)
	hst.Start()

	err = hst.Connect(t.ctx, t.bootstrapper)
	require.NoError(t.T, err)

	dht, err := dht.New(t.ctx, hst,
		dht.Mode(dht.ModeServer),
		dht.ProtocolPrefix("/test"),
		// needed to reduce connections to peers on DHT level
		dht.BucketSize(1),
	)
	require.NoError(t.T, err)

	err = dht.Bootstrap(t.ctx)
	require.NoError(t.T, err)

	return hst, routing.NewRoutingDiscovery(dht)
}

func checkPeer(t *testing.T, expected peer.ID, done chan struct{}) func(peerID peer.ID, isAdded bool) {
	return func(peerID peer.ID, isAdded bool) {
		defer close(done)
		require.Equal(t, expected, peerID)
		require.True(t, isAdded)
	}
}
