package light

import (
	"github.com/celestiaorg/celestia-node/pruner"
)

// Window is the availability window for light nodes in the Celestia
// network (30 days).
const Window = pruner.AvailabilityWindow(30 * 24 * 60 * 60)
