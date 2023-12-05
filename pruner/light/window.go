package light

import (
	"time"

	"github.com/celestiaorg/celestia-node/pruner"
)

const Window = pruner.AvailabilityWindow(time.Second * 86400 * 30)
