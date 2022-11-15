package das

import (
	"github.com/celestiaorg/celestia-node/das"
)

// WithMetrics is a utility function that is expected to be
// "invoked" by the fx lifecycle.
func WithMetrics(d *das.DASer) error {
	return d.InitMetrics()
}
