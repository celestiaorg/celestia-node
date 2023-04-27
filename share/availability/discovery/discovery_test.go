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
	mocknet "github.com/libp2p/go-libp2p/p2p/net/mock"
	"github.com/stretchr/testify/require"
)

func TestDiscovery(t *testing.T) {
	const fulls = 10

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	t.Cleanup(cancel)

	tn := newTestnet(ctx, t)

	peerA := tn.discovery(Parameters{
		PeersLimit:        fulls,
		DiscoveryInterval: time.Millisecond * 100,
		AdvertiseInterval: -1,
	})

	for range make([]int, fulls) {
		tn.discovery(Parameters{
			PeersLimit:        0,
			DiscoveryInterval: time.Hour,
			AdvertiseInterval: time.Millisecond * 100, // should only happen once
		})
	}

	var peers []peer.ID
	for len(peers) != fulls {
		peers, _ = peerA.Peers(ctx)
		if ctx.Err() != nil {
			t.Fatal("did not discover peers in time")
		}
	}
}

type testnet struct {
	ctx context.Context
	T   *testing.T
	net mocknet.Mocknet

	bootstrapper peer.ID
}

func newTestnet(ctx context.Context, t *testing.T) *testnet {
	net := mocknet.New()
	t.Cleanup(func() {
		err := net.Close()
		require.NoError(t, err)
	})

	hst, err := net.GenPeer()
	require.NoError(t, err)

	dht, err := dht.New(ctx, hst,
		dht.Mode(dht.ModeServer),
		dht.BootstrapPeers(),
		dht.ProtocolPrefix("/test"),
		dht.BucketSize(1),
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		err := dht.Close()
		require.NoError(t, err)
	})

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

	dht, err := dht.New(t.ctx, hst,
		dht.Mode(dht.ModeServer),
		dht.BootstrapPeers(peer.AddrInfo{ID: t.bootstrapper}),
		dht.ProtocolPrefix("/test"),
		dht.BucketSize(1),
	)
	require.NoError(t.T, err)
	t.T.Cleanup(func() {
		err := dht.Close()
		require.NoError(t.T, err)
	})

	err = dht.Bootstrap(t.ctx)
	require.NoError(t.T, err)

	return hst, routing.NewRoutingDiscovery(dht)
}
