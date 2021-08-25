// TODO: Ideally, we should abstract away FX, however, that's not that urgent
package node

import (
	"context"

	logging "github.com/ipfs/go-log/v2"
	"go.uber.org/fx"

	"github.com/celestiaorg/celestia-node/node/config"
)

var log = logging.Logger("node")

type StopFunc func(context.Context) error

// TODO
//  * Repo
//  * Bootstrapper
//  * Libp2p base
type Node struct {
	cfg *config.Config

	app *fx.App
}

func New(cfg *config.Config, opts ...fx.Option) (*Node, error) {
	node := &Node{
		cfg: cfg,
	}

	node.app = fx.New(
		fx.NopLogger,
		fx.Extract(node),
		fx.Options(opts...),
	)
	if err := node.app.Err(); err != nil {
		return nil, err
	}

	// TODO: Add bootstrapping
	return node, nil
}

func (n *Node) Start(ctx context.Context) error {
	return n.app.Start(ctx)
}

func (n *Node) Stop(ctx context.Context) error {
	return n.app.Stop(ctx)
}
