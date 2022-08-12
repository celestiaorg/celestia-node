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

// SetNodeType sets Node Type to the Env.
func SetNodeType(ctx context.Context, tp node.Type) context.Context {
	return context.WithValue(ctx, nodeTypeKey{}, tp)
}

// SetStorePath sets Store Path to the Env.
func SetStorePath(ctx context.Context, storePath string) context.Context {
	return context.WithValue(ctx, storePathKey{}, storePath)
}

// Options returns Node Options parsed from Environment(Flags, ENV vars, etc)
func Options(ctx context.Context) []node.Option {
	options, ok := ctx.Value(optionsKey{}).([]node.Option)
	if !ok {
		return []node.Option{}
	}
	return options
}

// AddOptions add new options to Env.
func AddOptions(ctx context.Context, opts ...node.Option) context.Context {
	options := Options(ctx)
	return context.WithValue(ctx, optionsKey{}, append(options, opts...))
}

type (
	optionsKey   struct{}
	storePathKey struct{}
	nodeTypeKey  struct{}
)
