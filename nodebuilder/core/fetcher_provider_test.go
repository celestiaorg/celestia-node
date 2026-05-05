package core

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx/fxtest"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	internalcore "github.com/celestiaorg/celestia-node/core"
)

// dialDummy returns a real *grpc.ClientConn that points at an unbound
// address. provideFetcher does not dial — it only constructs a client
// stub — so the connection never has to be valid.
func dialDummy(t *testing.T) *grpc.ClientConn {
	t.Helper()
	conn, err := grpc.NewClient(
		net.JoinHostPort("127.0.0.1", "1"), // arbitrary, never dialed
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

func TestProvideFetcher_SingleEndpointShortCircuits(t *testing.T) {
	lc := fxtest.NewLifecycle(t)
	cfg := DefaultConfig()
	cfg.IP = "127.0.0.1"

	primary := dialDummy(t)
	got, err := provideFetcher(lc, cfg, primary, AdditionalCoreConns{})
	require.NoError(t, err)

	_, ok := got.(*internalcore.BlockFetcher)
	assert.True(t, ok,
		"single-endpoint config must return a plain *BlockFetcher with no tracker overhead")
}

func TestProvideFetcher_MultiEndpointReturnsMultiFetcher(t *testing.T) {
	lc := fxtest.NewLifecycle(t)
	cfg := DefaultConfig()
	cfg.IP = "127.0.0.1"
	cfg.AdditionalCoreEndpoints = []EndpointConfig{
		{IP: "10.0.0.1", Port: "9090"},
		{IP: "10.0.0.2", Port: "9090", Archival: true},
	}

	primary := dialDummy(t)
	additional := AdditionalCoreConns{dialDummy(t), dialDummy(t)}

	got, err := provideFetcher(lc, cfg, primary, additional)
	require.NoError(t, err)

	_, ok := got.(*internalcore.MultiBlockFetcher)
	assert.True(t, ok, "multi-endpoint config must return *MultiBlockFetcher")
}

func TestProvideFetcher_RejectsConnConfigLengthMismatch(t *testing.T) {
	lc := fxtest.NewLifecycle(t)
	cfg := DefaultConfig()
	cfg.IP = "127.0.0.1"
	cfg.AdditionalCoreEndpoints = []EndpointConfig{
		{IP: "10.0.0.1", Port: "9090"},
	}

	primary := dialDummy(t)
	// length 2 but config declares only 1 — must error rather than silently
	// associate wrong configs with wrong conns.
	additional := AdditionalCoreConns{dialDummy(t), dialDummy(t)}

	_, err := provideFetcher(lc, cfg, primary, additional)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "length mismatch")
}

func TestProvideFetcher_LabelFallback(t *testing.T) {
	cfg := EndpointConfig{IP: "10.0.0.1", Port: "9090"}
	assert.Equal(t, "10.0.0.1:9090", labelFor(cfg))

	cfg.Label = "primary-mainnet"
	assert.Equal(t, "primary-mainnet", labelFor(cfg))
}

