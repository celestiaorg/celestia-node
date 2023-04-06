package p2p

import (
	"context"
	"crypto/rand"
	"testing"
	"time"

	"github.com/ipfs/go-datastore"
	"github.com/libp2p/go-libp2p"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	libhost "github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/metrics"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/protocol"
	rcmgr "github.com/libp2p/go-libp2p/p2p/host/resource-manager"
	mocknet "github.com/libp2p/go-libp2p/p2p/net/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestP2PModule_Host tests P2P Module methods on
// the instance of Host.
func TestP2PModule_Host(t *testing.T) {
	net, err := mocknet.FullMeshConnected(2)
	require.NoError(t, err)
	host, peer := net.Hosts()[0], net.Hosts()[1]

	mgr := newModule(host, nil, nil, nil, nil)

	ctx := context.Background()

	// test all methods on `manager.host`
	peers, err := mgr.Peers(ctx)
	require.NoError(t, err)
	assert.Equal(t, host.Network().Peers(), peers)

	peerInfo, err := mgr.PeerInfo(ctx, peer.ID())
	require.NoError(t, err)
	assert.Equal(t, libhost.InfoFromHost(peer).ID, peerInfo.ID)

	connectedness, err := mgr.Connectedness(ctx, peer.ID())
	require.NoError(t, err)
	assert.Equal(t, host.Network().Connectedness(peer.ID()), connectedness)
	// now disconnect using manager and check for connectedness match again
	assert.NoError(t, mgr.ClosePeer(ctx, peer.ID()))
	connectedness, err = mgr.Connectedness(ctx, peer.ID())
	require.NoError(t, err)
	assert.Equal(t, host.Network().Connectedness(peer.ID()), connectedness)
}

// TestP2PModule_ConnManager tests P2P Module methods on
// the Host's ConnManager. Note that this test is constructed differently
// than the one above because mocknet does not provide a ConnManager to its
// mock peers.
func TestP2PModule_ConnManager(t *testing.T) {
	// make two full peers and connect them
	host, err := libp2p.New()
	require.NoError(t, err)

	peer, err := libp2p.New()
	require.NoError(t, err)

	mgr := newModule(host, nil, nil, nil, nil)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	err = mgr.Connect(ctx, *libhost.InfoFromHost(peer))
	require.NoError(t, err)

	err = mgr.Protect(ctx, peer.ID(), "test")
	require.NoError(t, err)
	protected, err := mgr.IsProtected(ctx, peer.ID(), "test")
	require.NoError(t, err)
	assert.True(t, protected)

	ok, err := mgr.Unprotect(ctx, peer.ID(), "test")
	require.False(t, ok)
	require.NoError(t, err)
	protected, err = mgr.IsProtected(ctx, peer.ID(), "test")
	require.NoError(t, err)
	assert.False(t, protected)
}

// TestP2PModule_Autonat tests P2P Module methods on
// the node's instance of AutoNAT.
func TestP2PModule_Autonat(t *testing.T) {
	host, err := libp2p.New(libp2p.EnableNATService())
	require.NoError(t, err)

	mgr := newModule(host, nil, nil, nil, nil)

	status, err := mgr.NATStatus(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, network.ReachabilityUnknown, status)
}

// TestP2PModule_Bandwidth tests P2P Module methods on
// the Host's bandwidth reporter.
func TestP2PModule_Bandwidth(t *testing.T) {
	bw := metrics.NewBandwidthCounter()
	host, err := libp2p.New(libp2p.BandwidthReporter(bw))
	require.NoError(t, err)

	protoID := protocol.ID("test")
	// define a buf size, so we know how many bytes to read
	bufSize := 1000

	// create a peer to connect to
	peer, err := libp2p.New(libp2p.BandwidthReporter(bw))
	require.NoError(t, err)

	// set stream handler on the host
	host.SetStreamHandler(protoID, func(stream network.Stream) {
		buf := make([]byte, bufSize)
		_, err := stream.Read(buf)
		require.NoError(t, err)

		_, err = stream.Write(buf)
		require.NoError(t, err)
	})

	mgr := newModule(host, nil, nil, bw, nil)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	// connect to the peer
	err = mgr.Connect(ctx, *libhost.InfoFromHost(peer))
	require.NoError(t, err)

	// check to ensure they're actually connected
	connectedness, err := mgr.Connectedness(ctx, peer.ID())
	require.NoError(t, err)
	require.Equal(t, network.Connected, connectedness)

	// open stream with host
	info, err := mgr.Info(ctx)
	require.NoError(t, err)
	stream, err := peer.NewStream(ctx, info.ID, protoID)
	require.NoError(t, err)

	// write to stream to increase bandwidth usage get some substantive
	// data to read from the bandwidth counter
	buf := make([]byte, bufSize)
	_, err = rand.Read(buf)
	require.NoError(t, err)
	_, err = stream.Write(buf)
	require.NoError(t, err)

	_, err = stream.Read(buf)
	require.NoError(t, err)

	// has to be ~2 seconds for the metrics reporter to collect the stats
	// in the background process
	time.Sleep(time.Second * 2)

	stats, err := mgr.BandwidthStats(ctx)
	require.NoError(t, err)
	assert.NotNil(t, stats)

	peerStat, err := mgr.BandwidthForPeer(ctx, peer.ID())
	require.NoError(t, err)
	assert.NotZero(t, peerStat.TotalIn)
	assert.Greater(t, int(peerStat.TotalIn), bufSize) // should be slightly more than buf size due negotiations, etc

	protoStat, err := mgr.BandwidthForProtocol(ctx, protoID)
	require.NoError(t, err)
	assert.NotZero(t, protoStat.TotalIn)
	assert.Greater(t, int(protoStat.TotalIn), bufSize) // should be slightly more than buf size due negotiations, etc
}

// TestP2PModule_Pubsub tests P2P Module methods on
// the instance of pubsub.
func TestP2PModule_Pubsub(t *testing.T) {
	net, err := mocknet.FullMeshConnected(5)
	require.NoError(t, err)

	host := net.Hosts()[0]

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	gs, err := pubsub.NewGossipSub(ctx, host)
	require.NoError(t, err)

	mgr := newModule(host, gs, nil, nil, nil)

	topicStr := "test-topic"

	topic, err := gs.Join(topicStr)
	require.NoError(t, err)

	// also join all peers on mocknet to topic
	for _, p := range net.Hosts()[1:] {
		newGs, err := pubsub.NewGossipSub(ctx, p)
		require.NoError(t, err)

		tp, err := newGs.Join(topicStr)
		require.NoError(t, err)
		_, err = tp.Subscribe()
		require.NoError(t, err)
	}

	err = topic.Publish(ctx, []byte("test"))
	require.NoError(t, err)

	// give for some peers to properly join the topic (this is necessary
	// anywhere where gossipsub is used in tests)
	time.Sleep(1 * time.Second)

	psPeers, err := mgr.PubSubPeers(context.Background(), topicStr)
	require.NoError(t, err)
	assert.Equal(t, len(topic.ListPeers()), len(psPeers))
}

// TestP2PModule_ConnGater tests P2P Module methods on
// the instance of ConnectionGater.
func TestP2PModule_ConnGater(t *testing.T) {
	gater, err := connectionGater(datastore.NewMapDatastore())
	require.NoError(t, err)

	mgr := newModule(nil, nil, gater, nil, nil)

	ctx := context.Background()

	assert.NoError(t, mgr.BlockPeer(ctx, "badpeer"))
	blocked, err := mgr.ListBlockedPeers(ctx)
	require.NoError(t, err)
	assert.Len(t, blocked, 1)

	assert.NoError(t, mgr.UnblockPeer(ctx, "badpeer"))
	blocked, err = mgr.ListBlockedPeers(ctx)
	require.NoError(t, err)
	assert.Len(t, blocked, 0)
}

// TestP2PModule_ResourceManager tests P2P Module methods on
// the resourceManager.
func TestP2PModule_ResourceManager(t *testing.T) {
	rm, err := rcmgr.NewResourceManager(rcmgr.NewFixedLimiter(rcmgr.DefaultLimits.AutoScale()))
	require.NoError(t, err)

	mgr := newModule(nil, nil, nil, nil, rm)

	state, err := mgr.ResourceState(context.Background())
	require.NoError(t, err)

	assert.NotNil(t, state)
}
