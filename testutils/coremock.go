package testutils

import (
	"github.com/celestiaorg/celestia-core/abci/example/kvstore"
	"github.com/celestiaorg/celestia-core/node"
	rpctest "github.com/celestiaorg/celestia-core/rpc/test"

	"github.com/celestiaorg/celestia-node/core"
)

// StartMockCoreNode starts a mock core node background process and returns
// it, as well as the protocol and remote address of the node, in that order.
func StartMockCoreNode() *node.Node {
	app := kvstore.NewApplication()
	app.RetainBlocks = 10
	return rpctest.StartTendermint(app, rpctest.SuppressStdout)
}

// MockCoreClient returns started Mock Core Client.
func MockCoreClient() core.Client {
	return core.NewEmbeddedFromNode(StartMockCoreNode())
}
