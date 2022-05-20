package core

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/abci/example/kvstore"
	"github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/config"
	tmservice "github.com/tendermint/tendermint/libs/service"
	rpctest "github.com/tendermint/tendermint/rpc/test"
)

const defaultRetainBlocks int64 = 50

// StartTestNode starts a mock Core node background process and returns it.
func StartTestNode(ctx context.Context, t *testing.T, app types.Application, cfg *config.Config) tmservice.Service {
	nd, _, err := rpctest.StartTendermint(ctx, cfg, app, rpctest.SuppressStdout)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, nd.Stop())
	})
	return nd
}

// StartTestKVApp starts Tendermint KVApp.
func StartTestKVApp(ctx context.Context, t *testing.T) (tmservice.Service, types.Application, *config.Config) {
	cfg := config.TestConfig()
	app := CreateKVStore(defaultRetainBlocks)
	return StartTestNode(ctx, t, app, cfg), app, cfg
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
func StartTestClient(ctx context.Context, t *testing.T) (tmservice.Service, Client) {
	nd, _, cfg := StartTestKVApp(ctx, t)
	protocol, ip := GetEndpoint(cfg)
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
func GetEndpoint(cfg *config.Config) (string, string) {
	endpoint := cfg.RPC.ListenAddress
	protocol, ip := endpoint[:3], endpoint[6:]
	return protocol, ip
}
