package pruner

import (
	"time"
)

type AvailabilityWindow time.Duration

func IsWithinAvailabilityWindow(t time.Time, window AvailabilityWindow) bool {
	return time.Since(t) <= time.Duration(window)
}
