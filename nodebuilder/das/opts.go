package das

import (
	"go.uber.org/fx"

	"github.com/celestiaorg/celestia-node/das"
)

func WithMetrics() fx.Option {
	return fx.Invoke(func(d *das.DASer) error {
		return d.InitMetrics()
	})
}
