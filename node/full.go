package node

import (
	"context"

	"go.uber.org/fx"

	"github.com/celestiaorg/celestia-node/node/config"
	"github.com/celestiaorg/celestia-node/node/p2p"
)

func NewFull(ctx context.Context, cfg *config.Config) (StopFunc, error) {
	node, err := New(cfg, Full(cfg))
	if err != nil {
		return nil, err
	}

	err = node.Start(ctx)
	if err != nil {
		return nil, err
	}

	return node.Stop, nil
}

func Full(cfg *config.Config) fx.Option {
	return fx.Options(
		fx.Provide(p2p.Host),
	)
}
