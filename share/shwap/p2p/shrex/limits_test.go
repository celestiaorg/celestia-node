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

	reserve := eds.ResponseSize(share.MaxSquareSize * 2)
	require.NoError(t, scope.ReserveMemory(reserve, network.ReservationPriorityAlways))
}

// TestSetResourceLimits_AdmitsWorstCaseStreamedResponses checks that the streamed response types
// fit the default per-stream memory limit at the largest square, which materializing them did not.
func TestSetResourceLimits_AdmitsWorstCaseStreamedResponses(t *testing.T) {
	const networkID = "test"

	limits := rcmgr.DefaultLimits
	libp2p.SetDefaultServiceLimits(&limits)
	SetResourceLimits(&limits, networkID)

	rmgr, err := rcmgr.NewResourceManager(rcmgr.NewFixedLimiter(limits.AutoScale()))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, rmgr.Close()) })

	for _, req := range []request{&shwap.NamespaceDataID{}, &shwap.RangeNamespaceDataID{}} {
		t.Run(req.Name(), func(t *testing.T) {
			scope, err := rmgr.OpenStream(peer.ID("test-peer"), network.DirInbound)
			require.NoError(t, err)
			defer scope.Done()
			require.NoError(t, scope.SetProtocol(ProtocolID(networkID, req.Name())))
			require.NoError(t, scope.SetService(serviceName))

			reserve := req.ResponseSize(share.MaxSquareSize * 2)
			require.NoError(t, scope.ReserveMemory(reserve, network.ReservationPriorityAlways))
		})
	}
}

// TestSetResourceLimits_StreamCountsIgnoreReservationSize guards against shrinking per-request
// reservations silently raising how many shrex streams the service admits.
func TestSetResourceLimits_StreamCountsIgnoreReservationSize(t *testing.T) {
	limits := rcmgr.DefaultLimits
	libp2p.SetDefaultServiceLimits(&limits)
	SetResourceLimits(&limits, "test")

	const memGiB = 4
	scaled := limits.Scale(memGiB<<30, 1024).ToPartialLimitConfig()
	want := rcmgr.LimitVal(serviceBaseStreams + serviceStreamIncrease*globalLimitMultiplier*memGiB)
	require.Equal(t, want, scaled.Service[serviceName].StreamsInbound)
}

func TestEdsResponseSizeIsStreamBuffer(t *testing.T) {
	var eds shwap.EdsID
	base := eds.ResponseSize(64)
	for _, edsSize := range []int{128, 512, share.MaxSquareSize * 2} {
		require.Equal(t, base, eds.ResponseSize(edsSize),
			"EDS reservation must not depend on square size (edsSize=%d)", edsSize)
	}
}
