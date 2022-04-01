package core

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/abci/example/kvstore"
	"github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/node"
	rpctest "github.com/tendermint/tendermint/rpc/test"
)

const defaultRetainBlocks int64 = 50

// StartTestNode starts a mock Core node background process and returns it.
func StartTestNode(t *testing.T, app types.Application) *node.Node {
	nd := rpctest.StartTendermint(app, rpctest.SuppressStdout, rpctest.RecreateConfig)
	t.Cleanup(func() {
		rpctest.StopTendermint(nd)
	})
	return nd
}

// StartTestKVApp starts Tendermint KVApp.
func StartTestKVApp(t *testing.T) (*node.Node, types.Application) {
	app := CreateKVStore(defaultRetainBlocks)
	return StartTestNode(t, app), app
}

// CreateKVStore creates a simple kv store app and gives the user
// ability to set desired amount of blocks to be retained.
func CreateKVStore(retainBlocks int64) *kvstore.Application {
	app := kvstore.NewApplication()
	app.RetainBlocks = retainBlocks
	return app
}

// StartTestClient returns a started remote Core node process, as well its
// mock Core Client.
func StartTestClient(t *testing.T) (*node.Node, Client) {
	nd, _ := StartTestKVApp(t)
	protocol, ip := GetEndpoint(nd)
	client, err := NewRemote(protocol, ip)
	require.NoError(t, err)
	t.Cleanup(func() {
		err := client.Stop()
		require.NoError(t, err)
	})
	err = client.Start()
	require.NoError(t, err)
	return nd, client
}

// GetEndpoint returns the protocol and ip of the remote node.
func GetEndpoint(remote *node.Node) (string, string) {
	endpoint := remote.Config().RPC.ListenAddress
	protocol, ip := endpoint[:3], endpoint[6:]
	return protocol, ip
}
