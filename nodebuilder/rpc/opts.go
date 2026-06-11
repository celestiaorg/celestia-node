package rpc

import (
	"github.com/celestiaorg/celestia-node/api/rpc"
)

// WithMetrics enables OTel metrics on the RPC server. Intended to be invoked
// by the fx lifecycle.
func WithMetrics(server *rpc.Server) error {
	return server.WithMetrics()
}
