package pruner

import (
	"time"
)

type AvailabilityWindow time.Duration

// IsWithinAvailabilityWindow checks whether the given timestamp is within the
// given AvailabilityWindow. If the window is disabled (0), it returns true for
// every timestamp.
func IsWithinAvailabilityWindow(t time.Time, window AvailabilityWindow) bool {
	if window == AvailabilityWindow(time.Duration(0)) {
		return true
	}
	return time.Since(t) <= time.Duration(window)
}
