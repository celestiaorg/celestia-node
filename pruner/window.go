package pruner

import (
	"time"
)

type AvailabilityWindow time.Duration

func IsWithinAvailabilityWindow(t time.Time, window AvailabilityWindow) bool {
	if window == AvailabilityWindow(time.Duration(0)) {
		return true
	}
	return time.Since(t) <= time.Duration(window)
}
