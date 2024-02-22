package full

import (
	"time"

	"github.com/celestiaorg/celestia-node/pruner/light"
)

// Window is the availability window for light nodes in the Celestia
// network (30 days).
const Window = time.Duration(light.Window) + time.Hour
