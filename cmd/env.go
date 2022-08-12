package cmd

import (
	"context"

	"github.com/celestiaorg/celestia-node/node/config"
)

// NodeType reads the node type from the context.
func NodeType(ctx context.Context) config.NodeType {
	return ctx.Value(nodeTypeKey{}).(config.NodeType)
}

// StorePath reads the store path from the context.
func StorePath(ctx context.Context) string {
	return ctx.Value(storePathKey{}).(string)
}

// WithNodeType sets the node type in the given context.
func WithNodeType(ctx context.Context, tp config.NodeType) context.Context {
	return context.WithValue(ctx, nodeTypeKey{}, tp)
}

// WithStorePath sets Store Path in the given context.
func WithStorePath(ctx context.Context, storePath string) context.Context {
	return context.WithValue(ctx, storePathKey{}, storePath)
}

// NodeOptions returns config options parsed from Environment(Flags, ENV vars, etc)
func NodeOptions(ctx context.Context) []config.Option {
	options, ok := ctx.Value(optionsKey{}).([]config.Option)
	if !ok {
		return []config.Option{}
	}
	return options
}

// WithNodeOptions add new options to Env.
func WithNodeOptions(ctx context.Context, opts ...config.Option) context.Context {
	options := NodeOptions(ctx)
	return context.WithValue(ctx, optionsKey{}, append(options, opts...))
}

type (
	optionsKey   struct{}
	storePathKey struct{}
	nodeTypeKey  struct{}
)
