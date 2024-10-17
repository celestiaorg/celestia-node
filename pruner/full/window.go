package full

import (
	"time"

	"github.com/celestiaorg/celestia-node/pruner/light"
)

// Window is the availability window for light nodes in the Celestia
// network (30 days + 1 hour).
const Window = light.Window + time.Hour
