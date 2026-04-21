package stateclient

import (
	"context"
	"errors"
	"testing"
	"time"

	core "github.com/cometbft/cometbft/types"
	tmservice "github.com/cosmos/cosmos-sdk/client/grpc/cmtservice"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	appstate "github.com/celestiaorg/celestia-app/v9/fibre/state"

	"github.com/celestiaorg/celestia-node/header/headertest"
	"github.com/celestiaorg/celestia-node/nodebuilder/p2p"
)

// stubABCI lets a test control the ABCIQuery response and observe call counts.
type stubABCI struct {
	calls int
	resp  *tmservice.ABCIQueryResponse
	err   error
}

func (s *stubABCI) ABCIQuery(
	context.Context, *tmservice.ABCIQueryRequest, ...grpc.CallOption,
) (*tmservice.ABCIQueryResponse, error) {
	s.calls++
	return s.resp, s.err
}

func newTestClient(t *testing.T, abci abciQuerier, network p2p.Network) *Client {
	t.Helper()
	c := &Client{
		store:        headertest.NewStore(t),
		network:      network,
		abciQueryCli: abci,
		hostCache:    make(map[string]hostCacheEntry),
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

func TestStartChainIDAndMismatch(t *testing.T) {
	ctx := context.Background()

	// headertest store generates chain-id "test"; matching network → Start ok.
	c := newTestClient(t, &stubABCI{}, p2p.Network("test"))
	require.NoError(t, c.Start(ctx))
	head, err := c.store.Head(ctx)
	require.NoError(t, err)
	require.Equal(t, head.ChainID(), c.ChainID())

	// Mismatch → Start errors.
	c2 := newTestClient(t, &stubABCI{}, p2p.Arabica)
	err = c2.Start(ctx)
	require.Error(t, err)
	require.Contains(t, err.Error(), "chain-id mismatch")
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
		require.Equal(t, 1, stub.calls)
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

func TestGetHostCacheReuse(t *testing.T) {
	stub := &stubABCI{err: errors.New("must not be called")}
	c := newTestClient(t, stub, p2p.Private)

	// Pre-seed the cache and verify a lookup returns the cached host without
	// hitting ABCI.
	cacheKey := "some-cons-addr"
	c.storeHostCache(cacheKey, "validator.example:9090")

	got, ok := c.lookupHostCache(cacheKey)
	require.True(t, ok)
	assert.EqualValues(t, "validator.example:9090", got)
	assert.Zero(t, stub.calls)
}

func TestHostCacheExpiry(t *testing.T) {
	c := newTestClient(t, &stubABCI{}, p2p.Private)
	c.hostCache["k"] = hostCacheEntry{host: "stale", expires: time.Now().Add(-time.Second)}

	_, ok := c.lookupHostCache("k")
	require.False(t, ok)
}
