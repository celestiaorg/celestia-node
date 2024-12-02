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
	if windowOverride != time.Duration(0) {
		window = windowOverride
	}
	if window == time.Duration(0) {
		return true
	}
	return time.Since(t) <= window
}

// alwaysAvailable is a flag that disables the availability window.
var alwaysAvailable = os.Getenv("CELESTIA_PRUNING_DISABLED") == "true"

var windowOverride time.Duration

func init() {
	durationRaw := os.Getenv("CELESTIA_OVERRIDE_AVAILABILITY_WINDOW")
	duration, err := time.ParseDuration(durationRaw)
	if err != nil {
		panic("failed to parse CELESTIA_OVERRIDE_AVAILABILITY_WINDOW: " + err.Error())
	}
	windowOverride = duration
}
