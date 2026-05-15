package stateclient

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"

	core "github.com/cometbft/cometbft/types"
	tmservice "github.com/cosmos/cosmos-sdk/client/grpc/cmtservice"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	appstate "github.com/celestiaorg/celestia-app/v9/fibre/state"
	"github.com/celestiaorg/celestia-app/v9/fibre/validator"

	"github.com/celestiaorg/celestia-node/header/headertest"
	"github.com/celestiaorg/celestia-node/nodebuilder/p2p"
)

// stubABCI lets a test control the ABCIQuery response and observe call counts.
// The call counter is atomic so it stays sound under prefetchHosts' parallel
// fan-out and TestConcurrentGetHost's parallel readers.
type stubABCI struct {
	calls atomic.Int64
	resp  *tmservice.ABCIQueryResponse
	err   error
}

func (s *stubABCI) ABCIQuery(
	context.Context, *tmservice.ABCIQueryRequest, ...grpc.CallOption,
) (*tmservice.ABCIQueryResponse, error) {
	s.calls.Add(1)
	return s.resp, s.err
}

func newTestClient(t *testing.T, abci abciQuerier, network p2p.Network) *Client {
	t.Helper()
	c := &Client{
		store:        headertest.NewStore(t),
		network:      network,
		abciQueryCli: abci,
		hostCache:    make(map[string]validator.Host),
	}
	return c
}

func TestHeadAndGetByHeight(t *testing.T) {
	ctx := context.Background()
	c := newTestClient(t, &stubABCI{}, p2p.Private)

	set, err := c.Head(ctx)
	require.NoError(t, err)
	require.NotNil(t, set.ValidatorSet)
	require.Greater(t, set.Height, uint64(0))

	got, err := c.GetByHeight(ctx, set.Height)
	require.NoError(t, err)
	require.Equal(t, set.Height, got.Height)
	require.Equal(t, set.Hash(), got.Hash())
}

func TestStartSeedsChainID(t *testing.T) {
	ctx := context.Background()

	c := newTestClient(t, &stubABCI{}, p2p.Network("test"))
	require.NoError(t, c.Start(ctx))
	head, err := c.store.Head(ctx)
	require.NoError(t, err)
	require.Equal(t, head.ChainID(), c.ChainID())
}

func TestVerifyPromiseNoop(t *testing.T) {
	c := newTestClient(t, &stubABCI{}, p2p.Private)
	got, err := c.VerifyPromise(context.Background(), &appstate.PaymentPromise{})
	require.NoError(t, err)
	require.Equal(t, appstate.VerifiedPromise{}, got)
}

func TestStopNoop(t *testing.T) {
	c := newTestClient(t, &stubABCI{}, p2p.Private)
	require.NoError(t, c.Stop(context.Background()))
}

func TestGetHostABCIErrors(t *testing.T) {
	ctx := context.Background()
	val := &core.Validator{Address: make([]byte, 20)}

	t.Run("transport error", func(t *testing.T) {
		stub := &stubABCI{err: errors.New("boom")}
		c := newTestClient(t, stub, p2p.Private)
		_, err := c.GetHost(ctx, val)
		require.ErrorContains(t, err, "abci query")
		require.Equal(t, int64(1), stub.calls.Load())
	})

	t.Run("non-zero code", func(t *testing.T) {
		stub := &stubABCI{resp: &tmservice.ABCIQueryResponse{Code: 1, Log: "nope"}}
		c := newTestClient(t, stub, p2p.Private)
		_, err := c.GetHost(ctx, val)
		require.ErrorContains(t, err, "non-zero code")
	})

	t.Run("missing proof ops", func(t *testing.T) {
		stub := &stubABCI{resp: &tmservice.ABCIQueryResponse{}}
		c := newTestClient(t, stub, p2p.Private)
		_, err := c.GetHost(ctx, val)
		require.ErrorContains(t, err, "missing proof ops")
	})
}

// TestGetHostCacheHitEvicts checks that a GetHost call hitting the
// prefetch-populated cache returns immediately, evicts the entry, and
// makes no ABCI call.
func TestGetHostCacheHitEvicts(t *testing.T) {
	stub := &stubABCI{}
	c := newTestClient(t, stub, p2p.Private)

	addr := make([]byte, 20)
	for i := range addr {
		addr[i] = 0xAA
	}
	val := &core.Validator{Address: addr}
	cacheKey := sdk.ConsAddress(addr).String()
	c.hostCache[cacheKey] = validator.Host("cached.example:9090")

	got, err := c.GetHost(context.Background(), val)
	require.NoError(t, err)
	require.Equal(t, validator.Host("cached.example:9090"), got)
	require.Zero(t, stub.calls.Load(), "cache hit must not touch ABCI")

	_, present := c.hostCache[cacheKey]
	require.False(t, present, "cache hit must evict the entry")
}

// TestGetHostSecondCallMissesCache checks that after a cache hit evicts
// an entry, the next call for the same validator falls through to a
// fresh ABCI query.
func TestGetHostSecondCallMissesCache(t *testing.T) {
	stub := &stubABCI{err: errors.New("forced miss")}
	c := newTestClient(t, stub, p2p.Private)

	addr := make([]byte, 20)
	for i := range addr {
		addr[i] = 0xBB
	}
	val := &core.Validator{Address: addr}
	cacheKey := sdk.ConsAddress(addr).String()
	c.hostCache[cacheKey] = validator.Host("cached.example:9090")

	got, err := c.GetHost(context.Background(), val)
	require.NoError(t, err)
	require.Equal(t, validator.Host("cached.example:9090"), got)
	require.Zero(t, stub.calls.Load(), "first call must be served from cache")

	_, err = c.GetHost(context.Background(), val)
	require.ErrorContains(t, err, "abci query")
	require.Equal(t, int64(1), stub.calls.Load(), "second call must query ABCI exactly once")
}

// TestStartPrefetchToleratesErrors verifies that Start completes
// successfully even when every per-validator ABCI query fails, leaving
// the cache empty. Failures here include both transport errors and
// malformed responses.
func TestStartPrefetchToleratesErrors(t *testing.T) {
	t.Run("transport error", func(t *testing.T) {
		stub := &stubABCI{err: errors.New("boom")}
		c := newTestClient(t, stub, p2p.Network("test"))
		require.NoError(t, c.Start(context.Background()))
		require.Empty(t, c.hostCache)
		require.Greater(t, stub.calls.Load(), int64(0), "prefetch must attempt at least one query")
	})

	t.Run("missing proof ops", func(t *testing.T) {
		stub := &stubABCI{resp: &tmservice.ABCIQueryResponse{}}
		c := newTestClient(t, stub, p2p.Network("test"))
		require.NoError(t, c.Start(context.Background()))
		require.Empty(t, c.hostCache)
		require.Greater(t, stub.calls.Load(), int64(0))
	})
}

// TestConcurrentGetHost exercises GetHost from many goroutines in
// parallel, each targeting a distinct cached validator. With -race this
// catches any unsoundness in the cache mutex / eviction path.
func TestConcurrentGetHost(t *testing.T) {
	stub := &stubABCI{err: errors.New("should not be reached")}
	c := newTestClient(t, stub, p2p.Private)

	const N = 50
	addrs := make([][]byte, N)
	for i := range N {
		addr := make([]byte, 20)
		addr[0] = byte(i)
		addrs[i] = addr
		c.hostCache[sdk.ConsAddress(addr).String()] = validator.Host(fmt.Sprintf("h%d", i))
	}

	var wg sync.WaitGroup
	wg.Add(N)
	for i := range N {
		go func(i int) {
			defer wg.Done()
			val := &core.Validator{Address: addrs[i]}
			got, err := c.GetHost(context.Background(), val)
			require.NoError(t, err)
			require.Equal(t, validator.Host(fmt.Sprintf("h%d", i)), got)
		}(i)
	}
	wg.Wait()

	require.Zero(t, stub.calls.Load(), "every call should hit cache")
	require.Empty(t, c.hostCache, "every cache hit should evict")
}

// TestNewClientInitializesHostCache guards against a regression where
// the production constructor forgets to make the hostCache map and
// prefetchHosts would panic on a nil-map write.
func TestNewClientInitializesHostCache(t *testing.T) {
	conn, err := grpc.NewClient(
		"passthrough:test",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	c := NewClient(headertest.NewStore(t), conn, p2p.Private)
	require.NotNil(t, c.hostCache)
}
