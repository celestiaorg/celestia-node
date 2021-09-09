package core

import (
	"github.com/celestiaorg/celestia-core/abci/example/kvstore"
	"github.com/celestiaorg/celestia-core/node"
	rpctest "github.com/celestiaorg/celestia-core/rpc/test"
)

// StartMockNode starts a mock core node background process and returns it.
func StartMockNode() *node.Node {
	app := kvstore.NewApplication()
	app.RetainBlocks = 10
	return rpctest.StartTendermint(app, rpctest.SuppressStdout)
}

// MockClient returns started Mock Core Client.
func MockClient() Client {
	return NewEmbeddedFromNode(StartMockNode())
}
