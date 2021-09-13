package core

import (
	"github.com/celestiaorg/celestia-core/abci/example/kvstore"
	"github.com/celestiaorg/celestia-core/node"
	rpctest "github.com/celestiaorg/celestia-core/rpc/test"
)

// StartMockNode starts a mock Core node background process and returns it.
func StartMockNode() *node.Node {
	app := kvstore.NewApplication()
	app.RetainBlocks = 10
	return rpctest.StartTendermint(app, rpctest.SuppressStdout, rpctest.RecreateConfig)
}

// MockEmbeddedClient returns a started mock Core Client.
func MockEmbeddedClient() Client {
	return NewEmbeddedFromNode(StartMockNode())
}

// StartRemoteClient returns a started mock Core Client
// from the remote Core node process.
func StartRemoteClient() (*node.Node, Client, error) {
	remote := StartMockNode()
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
