package cmd

import (
	"context"

	logging "github.com/ipfs/go-log/v2"
	"go.uber.org/fx"

	"github.com/celestiaorg/celestia-node/nodebuilder"
	"github.com/celestiaorg/celestia-node/nodebuilder/node"
	"github.com/celestiaorg/celestia-node/nodebuilder/p2p"
)

var log = logging.Logger("cmd")

// NodeType reads the node type from the context.
func NodeType(ctx context.Context) node.Type {
	return ctx.Value(nodeTypeKey{}).(node.Type)
}

// Network reads the node type from the context.
func Network(ctx context.Context) p2p.Network {
	return ctx.Value(networkKey{}).(p2p.Network)
}

// StorePath reads the store path from the context.
func StorePath(ctx context.Context) string {
	return ctx.Value(storePathKey{}).(string)
}

// NodeConfig reads the node config from the context.
func NodeConfig(ctx context.Context) nodebuilder.Config {
	cfg, ok := ctx.Value(configKey{}).(nodebuilder.Config)
	if !ok {
		nodeType := NodeType(ctx)
		cfg = *nodebuilder.DefaultConfig(nodeType)
	}
	return cfg
}

// WithNodeType sets the node type in the given context.
func WithNodeType(ctx context.Context, tp node.Type) context.Context {
	return context.WithValue(ctx, nodeTypeKey{}, tp)
}

// WithNetwork sets the network in the given context.
func WithNetwork(ctx context.Context, network p2p.Network) context.Context {
	return context.WithValue(ctx, networkKey{}, network)
}

// WithStorePath sets Store Path in the given context.
func WithStorePath(ctx context.Context, storePath string) context.Context {
	return context.WithValue(ctx, storePathKey{}, storePath)
}

// NodeOptions returns config options parsed from Environment(Flags, ENV vars, etc)
func NodeOptions(ctx context.Context) []fx.Option {
	options, ok := ctx.Value(optionsKey{}).([]fx.Option)
	if !ok {
		return []fx.Option{}
	}
	return options
}

// WithNodeOptions add new options to Env.
func WithNodeOptions(ctx context.Context, opts ...fx.Option) context.Context {
	options := NodeOptions(ctx)
	return context.WithValue(ctx, optionsKey{}, append(options, opts...))
}

// WithNodeConfig sets the node config in the Env.
func WithNodeConfig(ctx context.Context, config *nodebuilder.Config) context.Context {
	return context.WithValue(ctx, configKey{}, *config)
}

type (
	optionsKey   struct{}
	configKey    struct{}
	storePathKey struct{}
	nodeTypeKey  struct{}
	networkKey   struct{}
)
