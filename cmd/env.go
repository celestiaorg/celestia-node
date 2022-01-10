package cmd

import (
	"context"
	"fmt"

	"github.com/celestiaorg/celestia-node/node"
)

// Env is an environment for CLI commands.
// It can be used to:
// 1. Propagate values from parent to child commands.
// 2. To group common logic that multiple commands rely on.
// Usage can be extended.
type Env struct {
	NodeType  node.Type
	StorePath string

	opts []node.Option
}

// WithEnv wraps given ctx with Env.
func WithEnv(ctx context.Context) context.Context {
	_, err := GetEnv(ctx)
	if err == nil {
		panic("cmd: only one Env is allowed to be set in a ctx")
	}

	return context.WithValue(ctx, envCtxKey{}, &Env{})
}

// GetEnv takes Env from the given ctx, if any.
func GetEnv(ctx context.Context) (*Env, error) {
	env, ok := ctx.Value(envCtxKey{}).(*Env)
	if !ok {
		return nil, fmt.Errorf("cmd: Env is not set in ctx.Context")
	}

	return env, nil
}

// SetNodeType sets Node Type to the Env.
func (env *Env) SetNodeType(tp node.Type) {
	env.NodeType = tp
}

// Options returns Node Options parsed from Environment(Flags, ENV vars, etc)
func (env *Env) Options() []node.Option {
	return env.opts
}

// AddOptions add new options to Env.
func (env *Env) AddOptions(opts ...node.Option) {
	env.opts = append(env.opts, opts...)
}

// envCtxKey is a key used to identify Env on a ctx.Context.
type envCtxKey struct{}
