package node

import (
	"context"

	"github.com/celestiaorg/celestia-node/node/config"
)

func NewLight(ctx context.Context, cfg *config.Config) (StopFunc, error) {
	node, err := New(cfg)
	if err != nil {
		return nil, err
	}

	err = node.Start(ctx)
	if err != nil {
		return nil, err
	}

	return node.Stop, nil
}
