package core

import (
	"github.com/celestiaorg/celestia-core/abci/example/kvstore"
	"github.com/celestiaorg/celestia-core/config"
	"github.com/celestiaorg/celestia-core/node"
	rpctest "github.com/celestiaorg/celestia-core/rpc/test"
	"os"
	"testing"
)

func MockConfigDevMode() *Config {
	return config.ResetTestRoot("dev")
}

// MockConfig provides a testing configuration for embedded Core Client.
func MockConfig(t *testing.T) *Config {
	cfg := config.ResetTestRoot(t.Name())
	t.Cleanup(func() {
		os.RemoveAll(cfg.RootDir)
	})
	return cfg
}

// StartMockNode starts a mock Core node background process and returns it.
func StartMockNode(suppressStdout bool) *node.Node {
	app := kvstore.NewApplication()
	app.RetainBlocks = 10
	if suppressStdout {
		return rpctest.StartTendermint(app, rpctest.SuppressStdout, rpctest.RecreateConfig)
	}
	return rpctest.StartTendermint(app, rpctest.RecreateConfig)
}

// MockEmbeddedClient returns a started mock Core Client.
func MockEmbeddedClient() Client {
	return NewEmbeddedFromNode(StartMockNode(false))
}


// StartRemoteClient returns a started remote Core node process, as well its
// mock Core Client.
func StartRemoteClient() (*node.Node, Client, error) {
	remote := StartMockNode(true)
	protocol, ip := getRemoteEndpoint(remote)
	client, err := NewRemote(protocol, ip)
	return remote, client, err
}

// getRemoteEndpoint returns the protocol and ip of the remote node.
func getRemoteEndpoint(remote *node.Node) (string, string) {
	endpoint := remote.Config().RPC.ListenAddress
	// protocol = "tcp"
	protocol, ip := endpoint[:3], endpoint[6:]
	return protocol, ip
}
