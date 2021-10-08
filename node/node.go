package node

import (
	"context"

	exchange "github.com/ipfs/go-ipfs-exchange-interface"
	format "github.com/ipfs/go-ipld-format"
	logging "github.com/ipfs/go-log/v2"
	"github.com/libp2p/go-libp2p-core/connmgr"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/routing"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"go.uber.org/fx"

	"github.com/celestiaorg/celestia-node/core"
	"github.com/celestiaorg/celestia-node/service/block"
)

var log = logging.Logger("node")

// Node represents the core structure of a Celestia node. It keeps references to all Celestia-specific
// components and services in one place and provides flexibility to run a Celestia node in different modes.
// Currently supported modes:
// * Full
// * Light
type Node struct {
	Type   Type
	Config *Config

	// the Node keeps a reference to the DI App that controls the lifecycles of services registered on the Node.
	app *fx.App

	// CoreClient provides access to a Core node process.
	CoreClient core.Client `optional:"true"`

	// p2p components
	Host         host.Host
	ConnGater    connmgr.ConnectionGater
	Routing      routing.PeerRouting
	DataExchange exchange.Interface
	DAG          format.DAGService
	// p2p protocols
	PubSub *pubsub.PubSub
	// BlockService provides access to the node's Block Service
	// TODO @renaynay: I don't like the concept of storing individual services on the node,
	// TODO maybe create a struct in full.go that contains `FullServices` (all expected services to be running on a
	// TODO full node) and in light, same thing `LightServices` (all expected services to be running in a light node.
	// TODO `FullServices` can include `LightServices` + other services.
	BlockServ *block.Service `optional:"true"`
}

// New assembles a new Node with the given type 'tp' over Repository 'repo'.
func New(tp Type, repo Repository) (*Node, error) {
	cfg, err := repo.Config()
	if err != nil {
		return nil, err
	}

	switch tp {
	case Full:
		return newNode(fullComponents(cfg, repo))
	case Light:
		// TODO @renaynay @wondertan: hacky, but fx.Provide does not allow two values to be written,
		// therefore Full node always has Light type since it's provided in lightComponents
		components := []fx.Option{
			lightComponents(cfg, repo),
			fx.Provide(func() Type {
				return Light
			}),
		}
		return newNode(components...)
	default:
		panic("node: unknown Node Type")
	}
}

// Start launches the Node and all its components and services.
func (n *Node) Start(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, n.app.StartTimeout())
	defer cancel()

	log.Debugf("Starting %s Node...", n.Type)
	err := n.app.Start(ctx)
	if err != nil {
		log.Errorf("Error starting %s Node: %s", n.Type, err)
		return err
	}

	log.Infof("%s Node is started", n.Type)

	// TODO(@Wondertan): Print useful information about the node:
	//  * API address
	//  * Pubkey/PeerID
	//  * Host listening address

	return nil
}

// Run is a Start which blocks on the given context 'ctx' until it is canceled.
// If canceled, the Node is still in the running state and should be gracefully stopped via Stop.
func (n *Node) Run(ctx context.Context) error {
	err := n.Start(ctx)
	if err != nil {
		return err
	}

	<-ctx.Done()
	return ctx.Err()
}

// Stop shuts down the Node, all its running Components/Services and returns.
// Canceling the given context earlier 'ctx' unblocks the Stop and aborts graceful shutdown forcing remaining
// Components/Services to close immediately.
func (n *Node) Stop(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, n.app.StopTimeout())
	defer cancel()

	log.Debugf("Stopping %s Node...", n.Type)
	err := n.app.Stop(ctx)
	if err != nil {
		log.Errorf("Error stopping %s Node: %s", n.Type, err)
		return err
	}

	log.Infof("%s Node is stopped", n.Type)
	return nil
}

// newNode creates a new Node from given DI options.
// DI options allow initializing the Node with a customized set of components and services.
// NOTE: newNode is currently meant to be used privately to create various custom Node types e.g. full, unless we
// decide to give package users the ability to create custom node types themselves.
func newNode(opts ...fx.Option) (*Node, error) {
	node := new(Node)
	node.app = fx.New(
		fx.NopLogger,
		fx.Extract(node),
		fx.Options(opts...),
	)
	return node, node.app.Err()
}
