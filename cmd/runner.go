package cmd

import (
	"context"
	"os"
	"path/filepath"

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

func NewRunner(config *nodebuilder.Config) (*Runner, error) {
	return &Runner{
		config:  config,
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
	return nodebuilder.Init(
		*r.config,
		StorePath(ctx),
		NodeType(ctx),
	)
}

func (r *Runner) Start(ctx context.Context) error {
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
		return err
	}

	store, err := nodebuilder.OpenStore(storePath, ring)
	if err != nil {
		return err
	}

	node, err := nodebuilder.NewWithConfig(
		NodeType(ctx),
		Network(ctx),
		store,
		r.config,
		NodeOptions(ctx)...,
	)
	if err != nil {
		return err
	}

	r.store = store
	r.node = node

	return r.node.Start(ctx)
}

func (r *Runner) Stop(ctx context.Context) error {
	return r.node.Stop(ctx)
}
