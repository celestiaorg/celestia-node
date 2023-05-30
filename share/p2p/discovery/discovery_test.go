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
	swarmt "github.com/libp2p/go-libp2p/p2p/net/swarm/testing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDiscovery(t *testing.T) {
	const nodes = 10 // higher number brings higher coverage

	discoveryRetryTimeout = time.Millisecond * 100 // defined in discovery.go

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	t.Cleanup(cancel)

	tn := newTestnet(ctx, t)

	peerA := tn.discovery(
		WithPeersLimit(nodes),
		WithAdvertiseInterval(-1),
	)

	type peerUpdate struct {
		peerID  peer.ID
		isAdded bool
	}
	updateCh := make(chan peerUpdate)
	peerA.WithOnPeersUpdate(func(peerID peer.ID, isAdded bool) {
		updateCh <- peerUpdate{peerID: peerID, isAdded: isAdded}
	})

	discs := make([]*Discovery, nodes)
	for i := range discs {
		discs[i] = tn.discovery(WithPeersLimit(0), WithAdvertiseInterval(time.Millisecond*100))

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

type testnet struct {
	ctx context.Context
	T   *testing.T

	bootstrapper peer.AddrInfo
}

func newTestnet(ctx context.Context, t *testing.T) *testnet {
	swarm := swarmt.GenSwarm(t, swarmt.OptDisableTCP)
	hst, err := basic.NewHost(swarm, &basic.HostOpts{})
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

func (t *testnet) discovery(opts ...Option) *Discovery {
	hst, routingDisc := t.peer()
	disc := NewDiscovery(hst, routingDisc, opts...)
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
	swarm := swarmt.GenSwarm(t.T, swarmt.OptDisableTCP)
	hst, err := basic.NewHost(swarm, &basic.HostOpts{})
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
