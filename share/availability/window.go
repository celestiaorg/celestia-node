package availability

import (
	"os"
	"time"
)

const (
	RequestWindow = 30 * 24 * time.Hour
	StorageWindow = RequestWindow + time.Hour
)

// IsWithinWindow checks whether the given timestamp is within the
// given AvailabilityWindow. If the window is disabled (0), it returns true for
// every timestamp.
func IsWithinWindow(t time.Time, window time.Duration) bool {
	if alwaysAvailable {
		return true
	}
	if window == time.Duration(0) {
		return true
	}
	return time.Since(t) <= window
}

// alwaysAvailable is a flag that disables the availability window.
var alwaysAvailable = os.Getenv("CELESTIA_PRUNING_DISABLED") == "true"
