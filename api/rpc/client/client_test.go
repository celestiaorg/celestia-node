package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/filecoin-project/go-jsonrpc"
	"github.com/stretchr/testify/require"
)

type testNodeHandler struct{}

func (testNodeHandler) Ready(context.Context) (bool, error) {
	return true, nil
}

func TestClientCloseClosesConnections(t *testing.T) {
	server := jsonrpc.NewServer()
	server.Register("node", testNodeHandler{})
	httpServer := httptest.NewServer(server)
	t.Cleanup(httpServer.Close)

	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)

	client, err := newClient(ctx, wsURL(httpServer.URL), nil)
	require.NoError(t, err)

	ready, err := client.Node.Ready(ctx)
	require.NoError(t, err)
	require.True(t, ready)

	client.Close()
	require.NotPanics(t, client.Close)

	callCtx, cancelCall := context.WithTimeout(ctx, time.Second)
	t.Cleanup(cancelCall)
	_, err = client.Node.Ready(callCtx)
	require.Error(t, err)
}

func TestNewClientClosesConnectionsOnError(t *testing.T) {
	server := jsonrpc.NewServer()
	firstClosed := make(chan struct{})
	var connections atomic.Int32
	httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if connections.Add(1) > 1 {
			http.Error(w, "unavailable", http.StatusServiceUnavailable)
			return
		}
		server.ServeHTTP(w, r)
		close(firstClosed)
	}))
	t.Cleanup(httpServer.Close)

	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)

	client, err := newClient(ctx, wsURL(httpServer.URL), nil)
	require.Error(t, err)
	require.Nil(t, client)
	require.Eventually(t, func() bool {
		select {
		case <-firstClosed:
			return true
		default:
			return false
		}
	}, time.Second, time.Millisecond)
}

func wsURL(httpURL string) string {
	return "ws" + strings.TrimPrefix(httpURL, "http")
}
