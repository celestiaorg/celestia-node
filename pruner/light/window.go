package light

import (
	"time"
)

// Window is the availability window for light nodes in the Celestia
// network (30 days).
const Window = 30 * 24 * time.Hour
