package client

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/filecoin-project/go-jsonrpc"
	"github.com/stretchr/testify/require"
)

// TestNewReadClient_ClosesClientsOnPartialInitFailure verifies that when one of
// the json-rpc clients fails to initialize, the clients opened before it are
// closed instead of leaked.
func TestNewReadClient_ClosesClientsOnPartialInitFailure(t *testing.T) {
	// Restore the real client constructor after the test.
	orig := newJSONRPCClient
	t.Cleanup(func() { newJSONRPCClient = orig })

	var opened, closed int
	// Fail on the 3rd client (header); the first two (share, blobstream) must be
	// closed by the rollback.
	const failOn = 3
	newJSONRPCClient = func(
		_ context.Context, _, _ string, _ interface{}, _ http.Header,
	) (jsonrpc.ClientCloser, error) {
		opened++
		if opened == failOn {
			return nil, errors.New("boom")
		}
		return func() { closed++ }, nil
	}

	c, err := NewReadClient(context.Background(), ReadConfig{BridgeDAAddr: "http://localhost:1"})
	require.Error(t, err)
	require.Nil(t, c)

	require.Equal(t, failOn, opened, "should have attempted clients up to the failing one")
	require.Equal(t, failOn-1, closed, "all clients opened before the failure must be closed")
}

// TestNewReadClient_NoCloseOnSuccess verifies the rollback does not fire when
// all clients initialize successfully.
func TestNewReadClient_NoCloseOnSuccess(t *testing.T) {
	orig := newJSONRPCClient
	t.Cleanup(func() { newJSONRPCClient = orig })

	var closed int
	newJSONRPCClient = func(
		_ context.Context, _, _ string, _ interface{}, _ http.Header,
	) (jsonrpc.ClientCloser, error) {
		return func() { closed++ }, nil
	}

	c, err := NewReadClient(context.Background(), ReadConfig{BridgeDAAddr: "http://localhost:1"})
	require.NoError(t, err)
	require.NotNil(t, c)
	require.Zero(t, closed, "clients must not be closed on the success path")

	// the assembled closer should still close all of them on demand.
	require.NoError(t, c.Close())
	require.Equal(t, 4, closed)
}
