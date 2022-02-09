package node

import (
	"context"
	"fmt"
	"net/http"
	"time"

	exchange "github.com/ipfs/go-ipfs-exchange-interface"
	format "github.com/ipfs/go-ipld-format"
	logging "github.com/ipfs/go-log/v2"
	"github.com/libp2p/go-libp2p-core/connmgr"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/routing"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"go.uber.org/fx"

	"github.com/celestiaorg/celestia-node/core"
	"github.com/celestiaorg/celestia-node/das"
	"github.com/celestiaorg/celestia-node/node/fxutil"
	"github.com/celestiaorg/celestia-node/node/rpc"
	"github.com/celestiaorg/celestia-node/service/block"
	"github.com/celestiaorg/celestia-node/service/header"
	"github.com/celestiaorg/celestia-node/service/share"
)

const Timeout = time.Second * 15

var log = logging.Logger("node")

// Node represents the core structure of a Celestia node. It keeps references to all Celestia-specific
// components and services in one place and provides flexibility to run a Celestia node in different modes.
// Currently supported modes:
// * Bridge
// * Light
// * Full
type Node struct {
	Type   Type
	Config *Config

	// CoreClient provides access to a Core node process.
	CoreClient core.Client `optional:"true"`

	// RPCServer provides access to Node's exposed APIs.
	RPCServer *rpc.Server `optional:"true"`

	// p2p components
	Host         host.Host
	ConnGater    connmgr.ConnectionGater
	Routing      routing.PeerRouting
	DataExchange exchange.Interface
	DAG          format.DAGService
	// p2p protocols
	PubSub *pubsub.PubSub
	// BlockService provides access to the node's Block Service
	BlockServ  *block.Service  `optional:"true"`
	ShareServ  share.Service   // not optional
	HeaderServ *header.Service // not optional

	DASer *das.DASer `optional:"true"`

	// start and stop control ref internal fx.App lifecycle funcs to be called from Start and Stop
	start, stop lifecycleFunc
}

// New assembles a new Node with the given type 'tp' over Store 'store'.
func New(tp Type, store Store, options ...Option) (*Node, error) {
	cfg, err := store.Config()
	if err != nil {
		return nil, err
	}

	s := new(settings)
	for _, option := range options {
		if option != nil {
			err := option(cfg, s)
			if err != nil {
				return nil, err
			}
		}
	}

	switch tp {
	case Bridge:
		return newNode(bridgeComponents(cfg, store), s.overrides())
	case Light:
		return newNode(lightComponents(cfg, store), s.overrides())
	case Full:
		return newNode(fullComponents(cfg, store), s.overrides())
	default:
		panic("node: unknown Node Type")
	}
}

// Start launches the Node and all its components and services.
func (n *Node) Start(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, Timeout)
	defer cancel()

	err := n.start(ctx)
	if err != nil {
		log.Errorf("starting %s Node: %s", n.Type, err)
		return fmt.Errorf("node: failed to start: %w", err)
	}

	// TODO(@Wondertan): Print useful information about the node:
	//  * API/RPC address
	log.Infof("started %s Node", n.Type)

	addrs, err := peer.AddrInfoToP2pAddrs(host.InfoFromHost(n.Host))
	if err != nil {
		log.Errorw("Retrieving multiaddress information", "err", err)
		return err
	}
	fmt.Println("The p2p host is listening on:")
	for _, addr := range addrs {
		fmt.Println("* ", addr.String())
	}
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

func (n *Node) RegisterAPI(endpoint string, api http.Handler) error {
	if n.RPCServer == nil {
		return fmt.Errorf("RPC server does not exist")
	}
	n.RPCServer.RegisterHandler(endpoint, api)
	return nil
}

// Stop shuts down the Node, all its running Components/Services and returns.
// Canceling the given context earlier 'ctx' unblocks the Stop and aborts graceful shutdown forcing remaining
// Components/Services to close immediately.
func (n *Node) Stop(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, Timeout)
	defer cancel()

	err := n.stop(ctx)
	if err != nil {
		log.Errorf("Stopping %s Node: %s", n.Type, err)
		return err
	}

	log.Infof("stopped %s Node", n.Type)
	return nil
}

// newNode creates a new Node from given DI options.
// DI options allow initializing the Node with a customized set of components and services.
// NOTE: newNode is currently meant to be used privately to create various custom Node types e.g. Light, unless we
// decide to give package users the ability to create custom node types themselves.
func newNode(opts ...fxutil.Option) (*Node, error) {
	fopt, err := fxutil.ParseOptions(opts...)
	if err != nil {
		return nil, err
	}

	node := new(Node)
	app := fx.New(
		fx.NopLogger,
		fx.Extract(node),
		fopt,
	)
	if err := app.Err(); err != nil {
		return nil, err
	}

	node.start, node.stop = app.Start, app.Stop
	return node, nil
}

// lifecycleFunc defines a type for common lifecycle funcs.
type lifecycleFunc func(context.Context) error
