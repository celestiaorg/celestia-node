package nodebuilder

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/cristalhq/jwt"
	"github.com/ipfs/go-blockservice"
	exchange "github.com/ipfs/go-ipfs-exchange-interface"
	logging "github.com/ipfs/go-log/v2"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/routing"
	"github.com/libp2p/go-libp2p/p2p/net/conngater"
	"go.uber.org/fx"
	"go.uber.org/fx/fxevent"
	"go.uber.org/zap/zapcore"

	"github.com/celestiaorg/celestia-node/api/gateway"
	"github.com/celestiaorg/celestia-node/api/rpc"
	"github.com/celestiaorg/celestia-node/nodebuilder/blob"
	"github.com/celestiaorg/celestia-node/nodebuilder/das"
	"github.com/celestiaorg/celestia-node/nodebuilder/fraud"
	"github.com/celestiaorg/celestia-node/nodebuilder/header"
	"github.com/celestiaorg/celestia-node/nodebuilder/node"
	"github.com/celestiaorg/celestia-node/nodebuilder/p2p"
	"github.com/celestiaorg/celestia-node/nodebuilder/share"
	"github.com/celestiaorg/celestia-node/nodebuilder/state"
)

var (
	log   = logging.Logger("node")
	fxLog = logging.Logger("fx")
)

// Node represents the core structure of a Celestia node. It keeps references to all
// Celestia-specific components and services in one place and provides flexibility to run a
// Celestia node in different modes. Currently supported modes:
// * Bridge
// * Light
// * Full
type Node struct {
	fx.In `ignore-unexported:"true"`

	Type          node.Type
	Network       p2p.Network
	Bootstrappers p2p.Bootstrappers
	Config        *Config
	AdminSigner   jwt.Signer

	// rpc components
	RPCServer     *rpc.Server     // not optional
	GatewayServer *gateway.Server `optional:"true"`

	// p2p components
	Host         host.Host
	ConnGater    *conngater.BasicConnectionGater
	Routing      routing.PeerRouting
	DataExchange exchange.Interface
	BlockService blockservice.BlockService
	// p2p protocols
	PubSub *pubsub.PubSub
	// services
	ShareServ  share.Module  // not optional
	HeaderServ header.Module // not optional
	StateServ  state.Module  // not optional
	FraudServ  fraud.Module  // not optional
	BlobServ   blob.Module   // not optional
	DASer      das.Module    // not optional

	// start and stop control ref internal fx.App lifecycle funcs to be called from Start and Stop
	start, stop lifecycleFunc
}

// New assembles a new Node with the given type 'tp' over Store 'store'.
func New(tp node.Type, network p2p.Network, store Store, options ...fx.Option) (*Node, error) {
	cfg, err := store.Config()
	if err != nil {
		return nil, err
	}

	return NewWithConfig(tp, network, store, cfg, options...)
}

// NewWithConfig assembles a new Node with the given type 'tp' over Store 'store' and a custom
// config.
func NewWithConfig(tp node.Type, network p2p.Network, store Store, cfg *Config, options ...fx.Option) (*Node, error) {
	opts := append([]fx.Option{ConstructModule(tp, network, cfg, store)}, options...)
	return newNode(opts...)
}

// Start launches the Node and all its components and services.
func (n *Node) Start(ctx context.Context) error {
	to := timeout(n.Type)
	ctx, cancel := context.WithTimeout(ctx, to)
	defer cancel()

	err := n.start(ctx)
	if err != nil {
		log.Debugf("error starting %s Node: %s", n.Type, err)
		if errors.Is(err, context.DeadlineExceeded) {
			return fmt.Errorf("node: failed to start within timeout(%s): %w", to, err)
		}
		return fmt.Errorf("node: failed to start: %w", err)
	}

	log.Infof("\n\n/_____/  /_____/  /_____/  /_____/  /_____/ \n\nStarted celestia DA node \nnode "+
		"type: 	%s\nnetwork: 	%s\n\n/_____/  /_____/  /_____/  /_____/  /_____/ \n", strings.ToLower(n.Type.String()),
		n.Network)

	addrs, err := peer.AddrInfoToP2pAddrs(host.InfoFromHost(n.Host))
	if err != nil {
		log.Errorw("Retrieving multiaddress information", "err", err)
		return err
	}
	fmt.Println("The p2p host is listening on:")
	for _, addr := range addrs {
		fmt.Println("* ", addr.String())
	}
	fmt.Println()
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

// Stop shuts down the Node, all its running Modules/Services and returns.
// Canceling the given context earlier 'ctx' unblocks the Stop and aborts graceful shutdown forcing
// remaining Modules/Services to close immediately.
func (n *Node) Stop(ctx context.Context) error {
	to := timeout(n.Type)
	ctx, cancel := context.WithTimeout(ctx, to)
	defer cancel()

	err := n.stop(ctx)
	if err != nil {
		log.Debugf("error stopping %s Node: %s", n.Type, err)
		if errors.Is(err, context.DeadlineExceeded) {
			return fmt.Errorf("node: failed to stop within timeout(%s): %w", to, err)
		}
		return fmt.Errorf("node: failed to stop: %w", err)
	}

	log.Debugf("stopped %s Node", n.Type)
	return nil
}

// newNode creates a new Node from given DI options.
// DI options allow initializing the Node with a customized set of components and services.
// NOTE: newNode is currently meant to be used privately to create various custom Node types e.g.
// Light, unless we decide to give package users the ability to create custom node types themselves.
func newNode(opts ...fx.Option) (*Node, error) {
	node := new(Node)
	app := fx.New(
		fx.WithLogger(func() fxevent.Logger {
			zl := &fxevent.ZapLogger{Logger: fxLog.Desugar()}
			zl.UseLogLevel(zapcore.DebugLevel)
			return zl
		}),
		fx.Populate(node),
		fx.Options(opts...),
	)
	if err := app.Err(); err != nil {
		return nil, err
	}

	node.start, node.stop = app.Start, app.Stop
	return node, nil
}

// lifecycleFunc defines a type for common lifecycle funcs.
type lifecycleFunc func(context.Context) error

var DefaultLifecycleTimeout = time.Minute * 2

func timeout(tp node.Type) time.Duration {
	switch tp {
	case node.Light:
		return time.Second * 20
	default:
		return DefaultLifecycleTimeout
	}
}
