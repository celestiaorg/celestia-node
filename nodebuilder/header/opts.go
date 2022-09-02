package header

import (
	"go.uber.org/fx"

	"github.com/celestiaorg/celestia-node/header"
)

// TODO: Eventually we should have a per-module metrics option.
// WithMetrics enables metrics exporting for the node.
func WithMetrics(enable bool) fx.Option {
	if !enable {
		return fx.Options()
	}
	return fx.Options(fx.Invoke(header.MonitorHead))
}
