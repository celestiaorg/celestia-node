package rpc

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServer(t *testing.T) {
	server := NewServer(Config{
		Address: "0.0.0.0",
		Port:    "0",
	})

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	err := server.Start(ctx)
	require.NoError(t, err)

	// register ping handler
	ping := new(ping)
	server.RegisterHandlerFunc("/ping", ping.ServeHTTP, http.MethodGet)

	url := fmt.Sprintf("http://%s/ping", server.listener.Addr().String())

	resp, err := http.Get(url)
	require.NoError(t, err)

	buf, err := ioutil.ReadAll(resp.Body)
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
