package shrex

import (
	"testing"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	rcmgr "github.com/libp2p/go-libp2p/p2p/host/resource-manager"
	"github.com/stretchr/testify/require"
)

// TestSetResourceLimits_AllowsOutboundStreams guards against the regression
// where shrex protocol limits set only Streams/StreamsInbound and left
// StreamsOutbound at zero.
func TestSetResourceLimits_AllowsOutboundStreams(t *testing.T) {
	const networkID = "test"

	// Mirror bridgeResources: defaults + libp2p service defaults + shrex limits.
	limits := rcmgr.DefaultLimits
	libp2p.SetDefaultServiceLimits(&limits)
	SetResourceLimits(&limits, networkID)

	rmgr, err := rcmgr.NewResourceManager(rcmgr.NewFixedLimiter(limits.AutoScale()))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, rmgr.Close()) })

	const testPeer = peer.ID("test-peer")
	for _, newReq := range registry {
		proto := ProtocolID(networkID, newReq().Name())

		scope, err := rmgr.OpenStream(testPeer, network.DirOutbound)
		require.NoErrorf(t, err, "open outbound stream for %s", proto)

		// SetProtocol reserves the stream at the shrex protocol (and
		// protocol-peer) scope — this is where a zero StreamsOutbound bit.
		err = scope.SetProtocol(proto)
		require.NoErrorf(t, err, "outbound shrex stream must not be resource-exhausted for %s", proto)

		scope.Done()
	}
}
