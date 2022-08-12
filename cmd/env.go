package cmd

import (
	"context"

	"github.com/celestiaorg/celestia-node/node"
)

// NodeType reads the node.Type from the context.
func NodeType(ctx context.Context) node.Type {
	return ctx.Value(nodeTypeKey{}).(node.Type)
}

// StorePath reads the store path from the context.
func StorePath(ctx context.Context) string {
	return ctx.Value(storePathKey{}).(string)
}

// WithNodeType sets Node Type in the given context.
func WithNodeType(ctx context.Context, tp node.Type) context.Context {
	return context.WithValue(ctx, nodeTypeKey{}, tp)
}

// WithStorePath sets Store Path in the given context.
func WithStorePath(ctx context.Context, storePath string) context.Context {
	return context.WithValue(ctx, storePathKey{}, storePath)
}

// NodeOptions returns node options parsed from Environment(Flags, ENV vars, etc)
func NodeOptions(ctx context.Context) []node.Option {
	options, ok := ctx.Value(optionsKey{}).([]node.Option)
	if !ok {
		return []node.Option{}
	}
	return options
}

// WithNodeOptions add new options to Env.
func WithNodeOptions(ctx context.Context, opts ...node.Option) context.Context {
	options := NodeOptions(ctx)
	return context.WithValue(ctx, optionsKey{}, append(options, opts...))
}

type (
	optionsKey   struct{}
	storePathKey struct{}
	nodeTypeKey  struct{}
)
