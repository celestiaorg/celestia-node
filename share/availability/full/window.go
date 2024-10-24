package full

import (
	"time"

	"github.com/celestiaorg/celestia-node/share/availability/light"
)

// Window is the availability window for full nodes in the Celestia
// network (30 days + 1 hour).
const Window = light.Window + time.Hour

// DisableStorageWindow extends the availability window for the storage of the node
// to the entire chain (used only for archival storage nodes).
const DisableStorageWindow = time.Duration(0)
