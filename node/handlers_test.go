package node

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFull_RPCStatus(t *testing.T) {
	repo := MockRepository(t, DefaultConfig(Full))
	node, err := New(Full, repo)
	require.NoError(t, err)
	require.NotNil(t, node)
	require.NotNil(t, node.Host)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	err = node.Start(ctx)
	require.NoError(t, err)

	// expected result
	expectedStatus, err := json.Marshal(
		NewStatusMessage(
			node.Config.P2P.ListenAddresses,
			node.Config.P2P.Network,
		),
	)
	require.NoError(t, err)

	// get status
	resp, err := http.Get(fmt.Sprintf("http://%s/status", node.Config.RPC.ListenAddr))
	require.NoError(t, err)

	// read status
	actualStatus, err := ioutil.ReadAll(resp.Body)
	require.NoError(t, err)
	t.Cleanup(func() {
		resp.Body.Close()
	})
	assert.Equal(t, expectedStatus, actualStatus)

	ctx, cancel = context.WithCancel(context.Background())
	t.Cleanup(cancel)

	require.NoError(t, node.Stop(ctx))
}
