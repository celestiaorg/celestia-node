package node

import (
	"context"

	logging "github.com/ipfs/go-log/v2"
	"github.com/libp2p/go-libp2p-core/host"
	"go.uber.org/fx"

	"github.com/celestiaorg/celestia-node/node/config"
)

var log = logging.Logger("node")

// Node is a central structure in Celestia Node git repository. It keeps references to all Celestia specific
// Components/Service in one place and provides flexibility to create custom celestia node types from it.
// Currently supported Node types:
// * Full
// * Light
type Node struct {
	Type   Type
	Config *config.Config
	Host   host.Host

	// the Node keeps a reference to the DI App that controls the lifecycles of services registered on the Node.
	app *fx.App
}

// newNode creates a new Node from given DI options.
// DI options allow filling the Node with a customized set of Components/Services.
// NOTE: It's meant to be used privately to create various custom Node types e.g. full / light, unless we eventually
//  decide to give package users the ability to create custom node types themselves.
func newNode(opts ...fx.Option) (*Node, error) {
	node := new(Node)
	node.app = fx.New(
		fx.NopLogger,
		fx.Extract(node),
		fx.Options(opts...),
	)
	return node, node.app.Err()
}

// Start launches the Node and all the referenced Components/Services.
// Canceling given context aborts the start.
func (n *Node) Start(ctx context.Context) error {
	log.Debugf("Starting %s Node...", n.Type)
	err := n.app.Start(ctx)
	if err != nil {
		log.Errorf("Error starting %s Node: %s", n.Type, err)
		return err
	}

	log.Infof("%s Node is started", n.Type)

	// TODO: Add bootstrapping
	return nil
}

// Stop shutdowns the Node and all the referenced Components/Services.
// Canceling given context unblocks Stop and aborts graceful shutdown while forcing remaining Components/Services to
// close immediately.
func (n *Node) Stop(ctx context.Context) error {
	log.Debugf("Stopping %s Node...", n.Type)
	err := n.app.Stop(ctx)
	if err != nil {
		log.Errorf("Error stopping %s Node: %s", n.Type, err)
		return err
	}

	log.Infof("%s Node is stopped", n.Type)
	return nil
}
