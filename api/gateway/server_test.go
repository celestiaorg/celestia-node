package gateway

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServer(t *testing.T) {
	address, port := "localhost", "0"
	server := NewServer(address, port)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	err := server.Start(ctx)
	require.NoError(t, err)

	// register ping handler
	ping := new(ping)
	server.RegisterHandlerFunc("/ping", ping.ServeHTTP, http.MethodGet)

	url := fmt.Sprintf("http://%s/ping", server.ListenAddr())

	resp, err := http.Get(url)
	require.NoError(t, err)

	buf, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	t.Cleanup(func() {
		resp.Body.Close()
	})
	assert.Equal(t, "pong", string(buf))

	err = server.Stop(ctx)
	require.NoError(t, err)
}

// TestServer_contextLeakProtection tests to ensure a context
// deadline was added by the context wrapper middleware server-side.
func TestServer_contextLeakProtection(t *testing.T) {
	address, port := "localhost", "0"
	server := NewServer(address, port)
	server.RegisterMiddleware(wrapRequestContext)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	err := server.Start(ctx)
	require.NoError(t, err)

	// register ping handler
	ch := new(contextHandler)
	server.RegisterHandlerFunc("/ch", ch.ServeHTTP, http.MethodGet)

	url := fmt.Sprintf("http://%s/ch", server.ListenAddr())
	req, err := http.NewRequest(http.MethodGet, url, nil)
	require.NoError(t, err)

	cli := new(http.Client)

	originalCtx, originalCancel := context.WithDeadline(context.Background(), time.Now().Add(time.Minute))
	t.Cleanup(originalCancel)
	resp, err := cli.Do(req.WithContext(originalCtx))
	require.NoError(t, err)
	buf, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())

	dur := new(time.Time)
	err = dur.UnmarshalJSON(buf)
	require.NoError(t, err)
	assert.True(t, dur.After(time.Now()))
}

type ping struct{}

func (p ping) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	//nolint:errcheck
	w.Write([]byte("pong"))
}

type contextHandler struct{}

func (ch contextHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	deadline, ok := r.Context().Deadline()
	if !ok {
		w.Write([]byte("no deadline")) //nolint:errcheck
		return
	}
	bin, err := deadline.MarshalJSON()
	if err != nil {
		panic(err)
	}
	w.Write(bin) //nolint:errcheck
}
