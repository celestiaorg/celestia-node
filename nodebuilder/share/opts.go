package share

import (
	"github.com/celestiaorg/celestia-node/share/p2p/peers"
	"github.com/celestiaorg/celestia-node/share/p2p/shrexeds"
)

// WithPeerManagerMetrics is a utility function that is expected to be
// "invoked" by the fx lifecycle.
func WithPeerManagerMetrics(m *peers.Manager) error {
	return m.WithMetrics()
}

func WithShrexClientMetrics(c *shrexeds.Client) error {
	return c.WithMetrics()
}

func WithShrexServerMetrics(s *shrexeds.Server) error {
	return s.WithMetrics()
}
