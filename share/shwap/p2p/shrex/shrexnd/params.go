package shrexnd

import (
	"fmt"

	logging "github.com/ipfs/go-log/v2"

	"github.com/celestiaorg/celestia-node/share/shwap/p2p/shrex"
)

var log = logging.Logger("shrex/nd")

// Parameters is the set of parameters that must be configured for the shrex/eds protocol.
type Parameters = shrex.Parameters

func DefaultParameters() *Parameters {
	return shrex.DefaultParameters()
}

func (c *Client) WithMetrics() error {
	metrics, err := shrex.InitClientMetrics("nd")
	if err != nil {
		return fmt.Errorf("shrex/nd: init Metrics: %w", err)
	}
	c.metrics = metrics
	return nil
}

func (srv *Server) WithMetrics() error {
	metrics, err := shrex.InitServerMetrics("nd")
	if err != nil {
		return fmt.Errorf("shrex/nd: init Metrics: %w", err)
	}
	srv.metrics = metrics
	return nil
}
