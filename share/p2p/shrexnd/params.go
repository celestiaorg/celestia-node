package shrexnd

import (
	"fmt"

	logging "github.com/ipfs/go-log/v2"

	"github.com/celestiaorg/celestia-node/share/p2p"
)

const protocolString = "/shrex/nd/v0.0.3"

var log = logging.Logger("shrex/nd")

// Parameters is the set of parameters that must be configured for the shrex/eds protocol.
type Parameters = p2p.Parameters

func DefaultParameters() *Parameters {
	return p2p.DefaultParameters()
}

func (c *Client) WithMetrics() error {
	metrics, err := p2p.InitClientMetrics("nd")
	if err != nil {
		return fmt.Errorf("shrex/nd: init Metrics: %w", err)
	}
	c.metrics = metrics
	return nil
}

func (srv *Server) WithMetrics() error {
	metrics, err := p2p.InitServerMetrics("nd")
	if err != nil {
		return fmt.Errorf("shrex/nd: init Metrics: %w", err)
	}
	srv.metrics = metrics
	return nil
}
