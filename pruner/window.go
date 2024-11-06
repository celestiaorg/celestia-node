package pruner

import (
	"os"
	"time"
)

type AvailabilityWindow time.Duration

func (aw AvailabilityWindow) Duration() time.Duration {
	return time.Duration(aw)
}

// IsWithinAvailabilityWindow checks whether the given timestamp is within the
// given AvailabilityWindow. If the window is disabled (0), it returns true for
// every timestamp.
func IsWithinAvailabilityWindow(t time.Time, window AvailabilityWindow) bool {
	if alwaysAvailable {
		return true
	}
	if window.Duration() == time.Duration(0) {
		return true
	}
	return time.Since(t) <= window.Duration()
}

// alwaysAvailable is a flag that disables the availability window.
var alwaysAvailable = os.Getenv("CELESTIA_PRUNING_DISABLED") == "true"
