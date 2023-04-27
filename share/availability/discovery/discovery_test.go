package discovery

import (
	"context"
	"testing"
	"time"

	"github.com/celestiaorg/celestia-node/logs"
	logging "github.com/ipfs/go-log/v2"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/discovery"
	"github.com/libp2p/go-libp2p/core/event"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/discovery/routing"
	basic "github.com/libp2p/go-libp2p/p2p/host/basic"
	"github.com/libp2p/go-libp2p/p2p/net/connmgr"
	swarm "github.com/libp2p/go-libp2p/p2p/net/swarm/testing"
	"github.com/stretchr/testify/require"
)

func TestDiscovery(t *testing.T) {
	logging.SetLogLevel("share/discovery", "debug")
	// logging.SetDebugLogging()
	logs.SetDebugLogging()

	const fulls = 120
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	t.Cleanup(cancel)

	tn := newTestnet(ctx, t)

	peerA := tn.discovery(Parameters{
		PeersLimit:        fulls,
		DiscoveryInterval: time.Millisecond*100,
		AdvertiseInterval: -1,
	})

	for range make([]int, fulls) {
		tn.discovery(Parameters{
			PeersLimit:        0,
			DiscoveryInterval: time.Hour,
			AdvertiseInterval: time.Millisecond*100, // should only happen once
		})
	}


	// go func() {
	// 	for {
	// 		time.Sleep(time.Second)
	// 		t.Log(len(peerA.host.Network().Peers()))
	// 	}
	// }()

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

	bootstrapper host.Host
}

func newTestnet(ctx context.Context, t *testing.T) *testnet {
	swrm := swarm.GenSwarm(t, swarm.OptDisableTCP)
	hst, err := basic.NewHost(swrm, &basic.HostOpts{})
	require.NoError(t, err)
	hst.Start()

	sub, err := hst.EventBus().Subscribe(&event.EvtPeerConnectednessChanged{})
	require.NoError(t, err)

	go func() {
		for {
			evt := (<-sub.Out()).(event.EvtPeerConnectednessChanged)
			log.Debugw("evt", "peer", evt.Peer)
		}
	}()

	dht, err := dht.New(ctx, hst, dht.DisableAutoRefresh(), dht.ProtocolPrefix("/test"), dht.Mode(dht.ModeServer), dht.BootstrapPeers(), dht.BucketSize(3))
	require.NoError(t, err)
	t.Cleanup(func() {
		err := dht.Close()
		require.NoError(t, err)
	})

	return &testnet{ctx: ctx, T: t, bootstrapper: hst}
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
	swrm := swarm.GenSwarm(t.T, swarm.OptDisableTCP)
	cm, err := connmgr.NewConnManager(0, 1, connmgr.WithGracePeriod(time.Millisecond*10))
	require.NoError(t.T, err)

	newHost, err := basic.NewHost(swrm, &basic.HostOpts{
		ConnManager: cm,
	})
	require.NoError(t.T, err)
	newHost.Start()

	err = newHost.Connect(t.ctx, peer.AddrInfo{
		ID: t.bootstrapper.ID(),
		Addrs: t.bootstrapper.Addrs(),
	})
	require.NoError(t.T, err)

	dht, err := dht.New(t.ctx, newHost, dht.DisableAutoRefresh(), dht.ProtocolPrefix("/test"), dht.Mode(dht.ModeServer), dht.BucketSize(3))
	require.NoError(t.T, err)
	t.T.Cleanup(func() {
		err := dht.Close()
		require.NoError(t.T, err)
	})

	<-dht.RefreshRoutingTable()

	t.T.Log(len(newHost.Network().Peers()))

	return newHost, routing.NewRoutingDiscovery(dht)
}
