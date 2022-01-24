package core

import (
	"os"
	"testing"

	"github.com/tendermint/tendermint/abci/example/kvstore"
	"github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/node"
	rpctest "github.com/tendermint/tendermint/rpc/test"
)

const defaultRetainBlocks int64 = 10

// MockConfig provides a testing configuration for embedded Core Client.
func MockConfig(t *testing.T) *Config {
	cfg := config.ResetTestRoot(t.Name())
	t.Cleanup(func() {
		os.RemoveAll(cfg.RootDir)
	})
	return cfg
}

// StartMockNode starts a mock Core node background process and returns it.
func StartMockNode(app types.Application) *node.Node {
	return rpctest.StartTendermint(app, rpctest.SuppressStdout, rpctest.RecreateConfig)
}

func EphemeralMockEmbeddedClient(t *testing.T) Client {
	nd := StartMockNode(CreateKvStore(defaultRetainBlocks))
	t.Cleanup(func() {
		nd.Stop() //nolint:errcheck
		rpctest.StopTendermint(nd)
	})
	return NewEmbeddedFromNode(nd)
}

// CreateKvStore creates a simple kv store app and gives the user
// ability to set desired amount of blocks to be retained.
func CreateKvStore(retainBlocks int64) *kvstore.Application {
	app := kvstore.NewApplication()
	app.RetainBlocks = retainBlocks
	return app
}

// MockEmbeddedClient returns a started mock Core Client.
func MockEmbeddedClient() Client {
	return NewEmbeddedFromNode(StartMockNode(CreateKvStore(defaultRetainBlocks)))
}

// StartRemoteClient returns a started remote Core node process, as well its
// mock Core Client.
func StartRemoteClient() (*node.Node, Client, error) {
	remote := StartMockNode(CreateKvStore(defaultRetainBlocks))
	protocol, ip := getRemoteEndpoint(remote)
	client, err := NewRemote(protocol, ip)
	return remote, client, err
}

// StartRemoteCore starts a remote core and returns it's protocol and address
func StartRemoteCore() (*node.Node, string, string) {
	remote := StartMockNode(CreateKvStore(defaultRetainBlocks))
	protocol, ip := getRemoteEndpoint(remote)
	return remote, protocol, ip
}

// getRemoteEndpoint returns the protocol and ip of the remote node.
func getRemoteEndpoint(remote *node.Node) (string, string) {
	endpoint := remote.Config().RPC.ListenAddress
	// protocol = "tcp"
	protocol, ip := endpoint[:3], endpoint[6:]
	return protocol, ip
}
