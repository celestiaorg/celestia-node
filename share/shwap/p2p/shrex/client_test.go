package shrex

import (
	"context"
	"testing"
	"time"

	mocknet "github.com/libp2p/go-libp2p/p2p/net/mock"
	"github.com/stretchr/testify/require"

	libshare "github.com/celestiaorg/go-square/v4/share"

	"github.com/celestiaorg/celestia-node/share/shwap"
)

// TestClient_StreamOpenTimeout tests that a peer stalling on stream open fails
// fast, freeing the request budget for other peers instead of hanging until the
// caller's context expires.
func TestClient_StreamOpenTimeout(t *testing.T) {
	streamOpenTimeout = 100 * time.Millisecond
	t.Cleanup(func() { streamOpenTimeout = 5 * time.Second })

	net, err := mocknet.FullMeshLinked(2)
	require.NoError(t, err)
	hosts := net.Hosts()

	// Latency well above streamOpenTimeout stalls the stream open.
	for _, link := range net.LinksBetweenPeers(hosts[0].ID(), hosts[1].ID()) {
		link.SetOptions(mocknet.LinkOptions{Latency: time.Minute})
	}

	client, err := NewClient(DefaultClientParameters(), hosts[0])
	require.NoError(t, err)

	id, err := shwap.NewNamespaceDataID(1, libshare.RandomNamespace())
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)

	start := time.Now()
	_, _, err = client.Get(ctx, &id, &shwap.NamespaceData{}, hosts[1].ID())
	require.ErrorContains(t, err, "open stream")
	require.Less(t, time.Since(start), 2*time.Second)
}
