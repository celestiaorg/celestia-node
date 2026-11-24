package shrex_getter

import (
	"context"
	"sync/atomic"
	"testing"
	"testing/synctest"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/libp2p/go-libp2p/p2p/net/conngater"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/header/headertest"
	"github.com/celestiaorg/celestia-node/share/availability"
	"github.com/celestiaorg/celestia-node/share/shwap"
	"github.com/celestiaorg/celestia-node/share/shwap/p2p/shrex"
	"github.com/celestiaorg/celestia-node/share/shwap/p2p/shrex/peers"
)

func TestGetSamples_ConcurrencyAndCancellation(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		h := &blockingStreamHost{}
		client, err := shrex.NewClient(shrex.DefaultClientParameters(), h)
		require.NoError(t, err)
		gater, err := conngater.NewBasicConnectionGater(nil)
		require.NoError(t, err)
		manager, err := peers.NewManager(*peers.DefaultParameters(), h, gater, "test")
		require.NoError(t, err)
		manager.UpdateNodePool(peer.ID("test-peer"), true)
		getter := NewGetter(client, manager, manager, availability.RequestWindow)
		_, roots, _ := generateTestEDS(t)
		hdr := headertest.RandExtendedHeaderWithRoot(t, roots)
		coords := make([]shwap.SampleCoords, 4*maxConcurrentSampleRequests)
		done := make(chan error, 1)
		go func() {
			samples, err := getter.GetSamples(ctx, hdr, coords)
			if len(samples) != len(coords) {
				t.Errorf("got %d samples for %d coordinates", len(samples), len(coords))
			}
			done <- err
		}()
		synctest.Wait()
		require.EqualValues(t, maxConcurrentSampleRequests, h.calls.Load())
		cancel()
		synctest.Wait()
		require.ErrorIs(t, <-done, context.Canceled)
		require.EqualValues(t, maxConcurrentSampleRequests, h.calls.Load(), "queued fetches must not open streams after cancellation")
	})
}

type blockingStreamHost struct {
	host.Host
	calls atomic.Int32
}

func (h *blockingStreamHost) NewStream(ctx context.Context, _ peer.ID, _ ...protocol.ID) (network.Stream, error) {
	h.calls.Add(1)
	<-ctx.Done()
	return nil, ctx.Err()
}
