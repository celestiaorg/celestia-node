package share

import (
	"github.com/celestiaorg/celestia-node/share/p2p/peers"
)

// WithPeerManagerMetrics is a utility function that is expected to be
// "invoked" by the fx lifecycle.
func WithPeerManagerMetrics(m *peers.Manager) error {
	return m.WithMetrics()
}
