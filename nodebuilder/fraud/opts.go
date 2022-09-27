package fraud

import (
	"go.uber.org/fx"

	"github.com/celestiaorg/celestia-node/fraud"
)

// WithMetrics enables metrics exporting for the node.
func WithMetrics(enable bool) fx.Option {
	if !enable {
		return fx.Options()
	}
	return fx.Options(fx.Invoke(fraud.MonitorProofs(fraud.RegisteredProofTypes()...)))
}
