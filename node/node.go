package node

import (
	"context"

	logging "github.com/ipfs/go-log/v2"
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
	tp  Type
	cfg *config.Config

	// we keep reference to App to control Node's lifecycle
	app *fx.App
}

// newNode creates a new Node from given DI options.
// DI options allow filling the Node with customized set of Components/Services in the node.
// NOTE: It's meant to be used privately to create various custom Node types e.g. full/LightNode, unless we don't
//  decide to give package users an ability to create custom node types themselves.
func newNode(cfg *config.Config, opts ...fx.Option) (*Node, error) {
	node := &Node{
		cfg: cfg,
	}

	node.app = fx.New(
		fx.NopLogger,
		fx.Extract(node),
		fx.Options(opts...),
	)
	if err := node.app.Err(); err != nil {
		return nil, err
	}

	return node, nil
}

// Type informs the node type.
func (n *Node) Type() Type {
	return n.tp
}

// Start launches the Node and all the referenced Components/Services.
// Cancelling given context aborts the start.
func (n *Node) Start(ctx context.Context) error {
	log.Debugf("Starting %s Node...", n.Type())
	err := n.app.Start(ctx)
	if err != nil {
		log.Errorf("Error starting %s Node: %s", n.Type(), err)
		return err
	}

	log.Infof("%s Node is started", n.Type())
	return nil
}

// Stop shutdowns the Node and all the referenced Components/Services.
// Cancelling given context unblocks Stop and aborts graceful shutdown while forcing remaining Components/Services to
// close immediately.
func (n *Node) Stop(ctx context.Context) error {
	log.Debugf("Stopping %s Node...", n.Type())
	err := n.app.Stop(ctx)
	if err != nil {
		log.Errorf("Error stopping %s Node: %s", n.Type(), err)
		return err
	}

	log.Infof("%s Node is stopped", n.Type())
	return nil
}
