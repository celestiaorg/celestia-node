package full

import (
	"time"

	"github.com/celestiaorg/celestia-node/pruner"
	"github.com/celestiaorg/celestia-node/pruner/light"
)

// Window is the availability window for light nodes in the Celestia
// network (30 days + 1 hour).
const Window = pruner.AvailabilityWindow(time.Duration(light.Window) + (time.Minute * 10))
