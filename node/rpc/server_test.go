package rpc

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServer(t *testing.T) {
	server := NewServer()
	err := server.Start("127.0.0.1:0")
	require.NoError(t, err)

	// register ping handler
	server.RegisterHandler("/ping", ping{})

	url := fmt.Sprintf("http://%s/ping", server.listener.Addr().String())

	resp, err := http.Get(url)
	require.NoError(t, err)

	buf, err := ioutil.ReadAll(resp.Body)
	require.NoError(t, err)
	t.Cleanup(func() {
		resp.Body.Close()
	})
	assert.Equal(t, "pong", string(buf))

	err = server.Stop()
	require.NoError(t, err)
}

type ping struct{}

func (p ping) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	//nolint:errcheck
	w.Write([]byte("pong"))
}
