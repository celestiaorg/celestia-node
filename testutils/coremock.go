package testutils

import (
	"github.com/celestiaorg/celestia-core/abci/example/kvstore"
	"github.com/celestiaorg/celestia-core/node"
	rpctest "github.com/celestiaorg/celestia-core/rpc/test"
)

// StartMockCoreNode starts a mock core node background process and returns
// it, as well as the protocol and remote address of the node, in that order.
func StartMockCoreNode() (*node.Node, string, string) {
	app := kvstore.NewApplication()
	app.RetainBlocks = 10

	mockNode := rpctest.StartTendermint(app, rpctest.SuppressStdout)

	endpoint := mockNode.Config().RPC.ListenAddress
	protocol, ip := endpoint[:3], endpoint[6:]

	return mockNode, protocol, ip
}
