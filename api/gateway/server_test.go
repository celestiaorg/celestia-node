package gateway

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/libs/net"
)

func TestServer(t *testing.T) {
	address := "localhost"
	port, err := net.GetFreePort()
	require.NoError(t, err)
	server := NewServer(address, strconv.Itoa(port))

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	err = server.Start(ctx)
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

type ping struct{}

func (p ping) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	//nolint:errcheck
	w.Write([]byte("pong"))
}
