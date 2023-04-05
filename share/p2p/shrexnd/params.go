package shrexnd

import (
	logging "github.com/ipfs/go-log/v2"

	"github.com/celestiaorg/celestia-node/share/p2p"
)

const protocolString = "/shrex/nd/0.0.1"

var log = logging.Logger("shrex/nd")

// Parameters is the set of parameters that must be configured for the shrex/eds protocol.
type Parameters = p2p.Parameters

func DefaultParameters() *Parameters {
	return p2p.DefaultParameters()
}
