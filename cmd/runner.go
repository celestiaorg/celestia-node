package cmd

import (
	"context"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/celestiaorg/celestia-app/app"
	"github.com/celestiaorg/celestia-app/app/encoding"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"

	"github.com/celestiaorg/celestia-node/nodebuilder"
)

// Runner is a wrapper for the various celestia
type Runner struct {
	config  *nodebuilder.Config
	store   nodebuilder.Store
	node    *nodebuilder.Node
	errChan chan error
}

func NewRunner(ctx context.Context) (*Runner, error) {
	config := NodeConfig(ctx)

	return &Runner{
		config:  &config,
		errChan: make(chan error, 1),
	}, nil
}

func (r *Runner) Errors() <-chan error {
	return r.errChan
}

func (r *Runner) Finish() {
	close(r.errChan)
}

func (r *Runner) Init(ctx context.Context) error {
	config := NodeConfig(ctx)
	return nodebuilder.Init(config, StorePath(ctx), NodeType(ctx))
}

func (r *Runner) Start(ctx context.Context) context.Context {
	ctx, cancel := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		defer cancel()

		storePath := StorePath(ctx)
		keysPath := filepath.Join(storePath, "keys")

		encConf := encoding.MakeConfig(app.ModuleEncodingRegisters...)
		ring, err := keyring.New(
			app.Name,
			r.config.State.KeyringBackend,
			keysPath,
			os.Stdin,
			encConf.Codec,
		)
		if err != nil {
			r.errChan <- err
			return
		}

		store, err := nodebuilder.OpenStore(storePath, ring)
		if err != nil {
			r.errChan <- err
			return
		}

		node, err := nodebuilder.NewWithConfig(
			NodeType(ctx),
			Network(ctx),
			store,
			r.config,
			NodeOptions(ctx)...,
		)
		if err != nil {
			r.errChan <- err
			return
		}

		r.store = store
		r.node = node

		err = r.node.Start(ctx)
		if err != nil {
			r.errChan <- err
			return
		}
	}()

	return ctx
}

func (r *Runner) Stop(ctx context.Context) <-chan struct{} {
	ctx, cancel := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		defer cancel()

		err := r.node.Stop(ctx)
		if err != nil {
			r.errChan <- err
			return
		}
	}()

	return ctx.Done()
}
