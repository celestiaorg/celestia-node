package cmd

import (
	"context"
	"fmt"

	"github.com/celestiaorg/celestia-node/node"
)

type Env struct {
	ndType    node.Type
	storePath string

	opts []node.Option
}

func WithEnv(ctx context.Context) context.Context {
	_, err := GetEnv(ctx)
	if err == nil {
		panic("cmd: only one Env is allowed to be set in a ctx")
	}

	return context.WithValue(ctx, envKey, &Env{})
}

func GetEnv(ctx context.Context) (*Env, error) {
	env, ok := ctx.Value(envKey).(*Env)
	if !ok {
		return nil, fmt.Errorf("cmd: Env is not set in ctx.Context")
	}

	return env, nil
}

func (env *Env) SetNodeType(tp node.Type) {
	env.ndType = tp
}

func (env *Env) Options() []node.Option {
	return env.opts
}

func (env *Env) addOption(opt node.Option) {
	env.opts = append(env.opts, opt)
}

type envCtxKey struct{}

var envKey = envCtxKey{}
