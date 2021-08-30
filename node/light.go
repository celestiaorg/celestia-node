package node

import (
	"context"

	"go.uber.org/fx"

	"github.com/celestiaorg/celestia-node/node/config"
	"github.com/celestiaorg/celestia-node/node/p2p"
)

// NewLight creates and starts a new ready-to-go Light Node.
// To gracefully stop it the Stop method must be used.
func NewLight(ctx context.Context, cfg *config.Config) (*Node, error) {
	node, err := newNode(cfg, light(cfg))
	if err != nil {
		return nil, err
	}

	err = node.Start(ctx)
	if err != nil {
		return nil, err
	}

	// TODO: Add bootstrapping

	node.tp = Light
	return node, nil
}

// light keeps all the DI options required to built a Light Node.
func light(cfg *config.Config) fx.Option {
	return fx.Options(
		fx.Provide(p2p.Host),
	)
}
