package discovery

import (
	"context"
	"testing"
	"time"

	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/discovery"
	"github.com/libp2p/go-libp2p/core/event"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/discovery/routing"
	mocknet "github.com/libp2p/go-libp2p/p2p/net/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDiscovery(t *testing.T) {
	const nodes = 30 // higher number brings higher coverage

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	t.Cleanup(cancel)

	tn := newTestnet(ctx, t)

	peerA := tn.discovery(Parameters{
		PeersLimit:        nodes,
		DiscoveryInterval: time.Millisecond * 100,
		AdvertiseInterval: -1,
	})
	sub, _ := peerA.host.EventBus().Subscribe(&event.EvtPeerConnectednessChanged{})

	discs := make([]*Discovery, nodes)
	for i := range discs {
		// start new node
		discs[i] = tn.discovery(Parameters{
			PeersLimit:        0,
			DiscoveryInterval: -1,
			AdvertiseInterval: time.Millisecond * 100,
		})

		// and check that we discover/connect to it
		select {
		case <-sub.Out():
		case <-ctx.Done():
			t.Fatal("did not discover peer in time")
		}
	}

	assert.Equal(t, peerA.set.Size(), nodes)

	// immediately cut peer from bootstrapper sp it cannot rediscover peers
	// helps with flakes
	// TODO: Check why backoff does not help
	err := peerA.host.Network().ClosePeer(tn.bootstrapper)
	require.NoError(t, err)

	for _, disc := range peerA.host.Network().Peers() {
		err := peerA.host.Network().ClosePeer(disc)
		require.NoError(t, err)

		select {
		case <-sub.Out():
		case <-ctx.Done():
			t.Fatal("did not disconnected peer in time")
		}
	}

	assert.Less(t, peerA.set.Size(), nodes)
}

type testnet struct {
	ctx context.Context
	T   *testing.T
	net mocknet.Mocknet

	bootstrapper peer.ID
}

func newTestnet(ctx context.Context, t *testing.T) *testnet {
	net := mocknet.New()
	hst, err := net.GenPeer()
	require.NoError(t, err)

	_, err = dht.New(ctx, hst,
		dht.Mode(dht.ModeServer),
		dht.BootstrapPeers(),
		dht.ProtocolPrefix("/test"),
	)
	require.NoError(t, err)

	return &testnet{ctx: ctx, T: t, net: net, bootstrapper: hst.ID()}
}

func (t *testnet) discovery(params Parameters) *Discovery {
	hst, routingDisc := t.peer()
	disc := NewDiscovery(hst, routingDisc, params)
	err := disc.Start(t.ctx)
	require.NoError(t.T, err)
	t.T.Cleanup(func() {
		err := disc.Stop(t.ctx)
		require.NoError(t.T, err)
	})

	go disc.Advertise(t.ctx)
	return disc
}

func (t *testnet) peer() (host.Host, discovery.Discovery) {
	hst, err := t.net.GenPeer()
	require.NoError(t.T, err)

	err = t.net.LinkAll()
	require.NoError(t.T, err)

	_, err = t.net.ConnectPeers(hst.ID(), t.bootstrapper)
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
