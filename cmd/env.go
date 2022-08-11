package cmd

import (
	"context"
	"github.com/celestiaorg/celestia-node/node"
)

// Env is an environment for CLI commands.
// It can be used to:
// 1. Propagate values from parent to child commands.
// 2. To group common logic that multiple commands rely on.
// Usage can be extended.
// TODO(@Wondertan): We should move to using context only instead.
//  Env, in fact, only keeps some additional fields which should be
//  kept in the context directly using WithValue (#965)

// NodeType reads the node.Type from the context.
func NodeType(ctx context.Context) node.Type {
	return ctx.Value("NodeType").(node.Type)
}

// StorePath reads the store path from the context.
func StorePath(ctx context.Context) string {
	return ctx.Value("StorePath").(string)
}

// SetNodeType sets Node Type to the Env.
func SetNodeType(ctx context.Context, tp node.Type) context.Context {
	return context.WithValue(ctx, "NodeType", tp)
}

// SetStorePath sets Store Path to the Env.
func SetStorePath(ctx context.Context, storePath string) context.Context {
	return context.WithValue(ctx, "StorePath", storePath)
}

// Options returns Node Options parsed from Environment(Flags, ENV vars, etc)
func Options(ctx context.Context) []node.Option {
	return ctx.Value("Options").([]node.Option)
}

// AddOptions add new options to Env.
func AddOptions(ctx context.Context, opts ...node.Option) context.Context {
	options := Options(ctx)
	return context.WithValue(ctx, "Options", append(options, opts...))
}
