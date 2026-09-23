package shrex

import (
	"testing"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	rcmgr "github.com/libp2p/go-libp2p/p2p/host/resource-manager"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/shwap"
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

// TestSetResourceLimits_AdmitsWorstCaseEDS checks that an EDS request for the largest square is
// admitted without touching the host-wide stream memory limit.
func TestSetResourceLimits_AdmitsWorstCaseEDS(t *testing.T) {
	const networkID = "test"

	limits := rcmgr.DefaultLimits
	libp2p.SetDefaultServiceLimits(&limits)
	streamMemory := limits.StreamBaseLimit.Memory
	SetResourceLimits(&limits, networkID)
	require.Equal(t, streamMemory, limits.StreamBaseLimit.Memory)

	rmgr, err := rcmgr.NewResourceManager(rcmgr.NewFixedLimiter(limits.AutoScale()))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, rmgr.Close()) })

	var eds shwap.EdsID
	proto := ProtocolID(networkID, eds.Name())
	scope, err := rmgr.OpenStream(peer.ID("test-peer"), network.DirInbound)
	require.NoError(t, err)
	defer scope.Done()
	require.NoError(t, scope.SetProtocol(proto))
	require.NoError(t, scope.SetService(serviceName))

	reserve := eds.ReserveSize(share.MaxSquareSize * 2)
	require.NoError(t, scope.ReserveMemory(reserve, network.ReservationPriorityAlways))
}

func TestEdsReserveSizeIsStreamBuffer(t *testing.T) {
	var eds shwap.EdsID
	base := eds.ReserveSize(64)
	for _, edsSize := range []int{128, 512, share.MaxSquareSize * 2} {
		require.Equal(t, base, eds.ReserveSize(edsSize),
			"EDS reservation must not depend on square size (edsSize=%d)", edsSize)
		require.Less(t, eds.ReserveSize(edsSize), eds.ResponseSize(edsSize),
			"EDS reservation must stay far below the wire size (edsSize=%d)", edsSize)
	}
}
