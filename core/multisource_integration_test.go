package core

import (
	"context"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/cometbft/cometbft/types"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/celestiaorg/go-header/p2p"
	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/header"
	nodep2p "github.com/celestiaorg/celestia-node/nodebuilder/p2p"
	"github.com/celestiaorg/celestia-node/store"
)

// These tests exercise MultiSource against a real celestia-app + CometBFT
// node (via createCoreFetcher) over real gRPC, rather than the in-memory
// fakeSource used by the unit tests. They cover the assumptions that unit tests
// can't: real chain-ID verification, that a dead endpoint does not stall a live
// one, and that deduplication holds end-to-end on real blocks delivered twice.

// deadGRPCClient returns a lazily-dialed connection to an address nothing
// listens on, so any RPC against it fails. It models a configured-but-down
// endpoint. No retry interceptors, so failures surface promptly.
func deadGRPCClient(t *testing.T) *grpc.ClientConn {
	t.Helper()
	conn, err := grpc.NewClient("127.0.0.1:1", grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

// secondClientTo opens a second gRPC connection to the same node as conn, so a
// MultiSource can fan two real subscriptions in from one chain.
func secondClientTo(t *testing.T, conn *grpc.ClientConn) *grpc.ClientConn {
	t.Helper()
	host, port, err := net.SplitHostPort(conn.Target())
	require.NoError(t, err)
	return newTestClient(t, host, port)
}

func TestMultiSource_Integration_Verify(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	t.Cleanup(cancel)

	cfg := DefaultTestConfig().WithChainID(testChainID)
	_, network := createCoreFetcher(t, cfg)
	good := network.GRPCClient

	t.Run("sources on the expected chain are confirmed and kept", func(t *testing.T) {
		ms := NewMultiSource(good, secondClientTo(t, good))
		require.NoError(t, ms.Verify(ctx, testChainID))
		assert.Len(t, ms.sources, 2)
	})

	t.Run("wrong expected chain ID refuses to start", func(t *testing.T) {
		ms := NewMultiSource(good)
		require.Error(t, ms.Verify(ctx, "not-"+testChainID))
	})

	t.Run("unreachable source is kept when another is confirmed", func(t *testing.T) {
		vctx, vcancel := context.WithTimeout(ctx, 10*time.Second)
		defer vcancel()
		ms := NewMultiSource(good, deadGRPCClient(t))
		require.NoError(t, ms.Verify(vctx, testChainID))
		assert.Len(t, ms.sources, 2, "unreachable endpoint kept so it can join later")
	})
}

func TestMultiSource_Integration_FanInToleratesDeadSource(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	t.Cleanup(cancel)

	cfg := DefaultTestConfig().WithChainID(testChainID)
	_, network := createCoreFetcher(t, cfg)

	// One live endpoint, one dead one: the live source must keep delivering real
	// blocks while the dead source's fetcher retries in the background.
	ms := NewMultiSource(network.GRPCClient, deadGRPCClient(t))
	out, err := ms.SubscribeNewBlockEvent(ctx)
	require.NoError(t, err)

	var last int64
	for range 3 {
		select {
		case b := <-out:
			require.NotNil(t, b.Header)
			assert.Greater(t, b.Header.Height, int64(0))
			assert.Greater(t, b.Header.Height, last, "live source delivers heights in order")
			last = b.Header.Height
		case <-ctx.Done():
			t.Fatal("no blocks from the live source while the other endpoint is dead")
		}
	}
}

func TestMultiSource_Integration_DedupThroughListener(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	t.Cleanup(cancel)

	cfg := DefaultTestConfig().WithChainID(testChainID)
	_, network := createCoreFetcher(t, cfg)

	// Two connections to the same node, so every height is fanned in twice and
	// the dedup gate is genuinely exercised on real blocks.
	ms := NewMultiSource(network.GRPCClient, secondClientTo(t, network.GRPCClient))

	ps0, ps1 := createMocknetWithTwoPubsubEndpoints(ctx, t)
	bcast := newTestSubscriber(ctx, t, ps0)
	observer := newTestSubscriber(ctx, t, ps1)
	subs, err := observer.Subscribe()
	require.NoError(t, err)
	t.Cleanup(subs.Cancel)

	edsSub := createEdsPubSub(ctx, t)
	st, err := store.NewStore(store.DefaultParameters(), t.TempDir())
	require.NoError(t, err)

	// Count construct calls per height: with sequential processing in listen(),
	// the dedup gate must keep each height to exactly one construction despite
	// both sources delivering it.
	var (
		mu     sync.Mutex
		counts = map[int64]int{}
	)
	construct := func(
		h *types.Header,
		c *types.Commit,
		vs *types.ValidatorSet,
		eds *rsmt2d.ExtendedDataSquare,
	) (*header.ExtendedHeader, error) {
		mu.Lock()
		counts[h.Height]++
		mu.Unlock()
		return header.MakeExtendedHeader(h, c, vs, eds)
	}

	cl, err := NewListener(bcast, ms, edsSub.Broadcast, construct, st, nodep2p.BlockTime,
		WithChainID(nodep2p.Network(testChainID)))
	require.NoError(t, err)
	require.NoError(t, cl.Start(ctx))

	// Wait until a handful of heights have been processed and broadcast.
	for range 5 {
		_, err := subs.NextHeader(ctx)
		require.NoError(t, err)
	}
	require.NoError(t, cl.Stop(ctx))

	mu.Lock()
	defer mu.Unlock()
	require.NotEmpty(t, counts)
	for h, n := range counts {
		assert.Equalf(t, 1, n,
			"height %d constructed %d times; dedup should build each height once despite two sources", h, n)
	}
}

// newTestSubscriber builds and starts a header pubsub subscriber on ps with a
// pass-through verifier, usable both as a broadcaster and an observer.
func newTestSubscriber(ctx context.Context, t *testing.T, ps *pubsub.PubSub) *p2p.Subscriber[*header.ExtendedHeader] {
	t.Helper()
	sub, err := p2p.NewSubscriber[*header.ExtendedHeader](ps, header.MsgID, p2p.WithSubscriberNetworkID(testChainID))
	require.NoError(t, err)
	require.NoError(t, sub.SetVerifier(func(context.Context, *header.ExtendedHeader) error { return nil }))
	require.NoError(t, sub.Start(ctx))
	t.Cleanup(func() { require.NoError(t, sub.Stop(ctx)) })
	return sub
}
